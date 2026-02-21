package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/lute/api/config"
	"github.com/lute/api/models"
	"github.com/lute/api/repository"
)

// AgentBinaryInfo describes one compiled agent binary
type AgentBinaryInfo struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
}

// AgentSetupRequest is sent by the agent during --setup to register a new machine
type AgentSetupRequest struct {
	Name      string            `json:"name" binding:"required"`
	Hostname  string            `json:"hostname"`
	OS        string            `json:"os"`
	Arch      string            `json:"arch"`
	CPUs      int               `json:"cpus"`
	IP        string            `json:"ip"`
	Version   string            `json:"version"`
	Metadata  map[string]string  `json:"metadata,omitempty"`
	ClaimCode string            `json:"claim_code,omitempty"` // optional; links machine to user when valid
}

// AgentSetupResponse is returned after the agent registers a new machine
type AgentSetupResponse struct {
	MachineID   string `json:"machine_id"`
	GRPCAddress string `json:"grpc_address"`
	Message     string `json:"message"`
}

// claimEntry holds a short-lived claim code that links a new machine to a user
type claimEntry struct {
	UserID    string
	ExpiresAt time.Time
}

// AgentHandler serves compiled agent binaries and handles agent registration
type AgentHandler struct {
	binaryDir   string                      // directory containing compiled agent binaries
	mu          sync.RWMutex                // protects the cache
	cache       map[string]*AgentBinaryInfo // key: "os/arch"
	claimMu     sync.RWMutex
	claimCodes  map[string]*claimEntry      // short-lived codes: code -> userID + expiry
	cfg         *config.Config
	machineRepo *repository.MachineRepository
	commandRepo *repository.CommandRepository
}

// NewAgentHandler creates a handler that serves agent binaries from binaryDir.
// binaryDir layout:
//
//	<binaryDir>/
//	  lute-agent-linux-amd64
//	  lute-agent-linux-arm64
//	  lute-agent-darwin-amd64
//	  lute-agent-darwin-arm64
//	  lute-agent-windows-amd64.exe
func NewAgentHandler(
	binaryDir string,
	cfg *config.Config,
	machineRepo *repository.MachineRepository,
	commandRepo *repository.CommandRepository,
) *AgentHandler {
	h := &AgentHandler{
		binaryDir:   binaryDir,
		cache:       make(map[string]*AgentBinaryInfo),
		claimCodes:  make(map[string]*claimEntry),
		cfg:         cfg,
		machineRepo: machineRepo,
		commandRepo: commandRepo,
	}
	h.refreshCache()
	return h
}

const claimCodeLen = 20
const claimCodeExpiry = 15 * time.Minute
const claimCodeChars = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ" // no I,O to avoid confusion

func (h *AgentHandler) createClaimCode(userID string) (code string, expiresAt time.Time) {
	b := make([]byte, claimCodeLen)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = claimCodeChars[int(b[i])%len(claimCodeChars)]
	}
	code = string(b)
	expiresAt = time.Now().Add(claimCodeExpiry)
	h.claimMu.Lock()
	defer h.claimMu.Unlock()
	// Remove expired
	for k, v := range h.claimCodes {
		if time.Now().After(v.ExpiresAt) {
			delete(h.claimCodes, k)
		}
	}
	h.claimCodes[code] = &claimEntry{UserID: userID, ExpiresAt: expiresAt}
	return code, expiresAt
}

func (h *AgentHandler) consumeClaimCode(code string) (userID string, ok bool) {
	if code == "" {
		return "", false
	}
	h.claimMu.Lock()
	defer h.claimMu.Unlock()
	ent, ok := h.claimCodes[code]
	if !ok || time.Now().After(ent.ExpiresAt) {
		return "", false
	}
	delete(h.claimCodes, code)
	return ent.UserID, true
}

// refreshCache scans the binary directory and rebuilds metadata cache
func (h *AgentHandler) refreshCache() {
	h.mu.Lock()
	defer h.mu.Unlock()

	entries, err := os.ReadDir(h.binaryDir)
	if err != nil {
		log.Printf("Warning: cannot read agent binary dir %s: %v", h.binaryDir, err)
		return
	}

	newCache := make(map[string]*AgentBinaryInfo)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "lute-agent-") {
			continue
		}

		osName, arch := parseFilename(name)
		if osName == "" || arch == "" {
			continue
		}

		fullPath := filepath.Join(h.binaryDir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		checksum, err := sha256File(fullPath)
		if err != nil {
			log.Printf("Warning: cannot compute checksum for %s: %v", name, err)
			continue
		}

		key := osName + "/" + arch
		newCache[key] = &AgentBinaryInfo{
			OS:       osName,
			Arch:     arch,
			Version:  readVersionFile(h.binaryDir),
			Filename: name,
			SHA256:   checksum,
			Size:     info.Size(),
		}
		log.Printf("Indexed agent binary: %s (%s/%s, %d bytes)", name, osName, arch, info.Size())
	}

	h.cache = newCache
}

// ListBinaries returns metadata about all available agent binaries
// GET /api/v1/agent/binaries
func (h *AgentHandler) ListBinaries(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	binaries := make([]*AgentBinaryInfo, 0, len(h.cache))
	for _, b := range h.cache {
		binaries = append(binaries, b)
	}

	c.JSON(http.StatusOK, gin.H{
		"binaries": binaries,
		"version":  readVersionFile(h.binaryDir),
	})
}

// DownloadBinary serves the agent binary for the requested OS/arch
// GET /api/v1/agent/download/:os/:arch
func (h *AgentHandler) DownloadBinary(c *gin.Context) {
	osName := c.Param("os")
	arch := c.Param("arch")
	key := osName + "/" + arch

	h.mu.RLock()
	info, ok := h.cache[key]
	h.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     fmt.Sprintf("no agent binary for %s/%s", osName, arch),
			"available": h.availableKeys(),
		})
		return
	}

	fullPath := filepath.Join(h.binaryDir, info.Filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", info.Filename))
	c.Header("X-Agent-Version", info.Version)
	c.Header("X-Agent-SHA256", info.SHA256)
	c.File(fullPath)
}

// DownloadAutoDetect serves the binary based on the requesting machine's info
// GET /api/v1/agent/download  (auto-detect from query: ?os=linux&arch=amd64)
func (h *AgentHandler) DownloadAutoDetect(c *gin.Context) {
	osName := c.DefaultQuery("os", "linux")
	arch := c.DefaultQuery("arch", "amd64")
	key := osName + "/" + arch

	h.mu.RLock()
	info, ok := h.cache[key]
	h.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     fmt.Sprintf("no agent binary for %s/%s", osName, arch),
			"available": h.availableKeys(),
		})
		return
	}

	fullPath := filepath.Join(h.binaryDir, info.Filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", info.Filename))
	c.Header("X-Agent-Version", info.Version)
	c.Header("X-Agent-SHA256", info.SHA256)
	c.File(fullPath)
}

// GetVersion returns the current agent version
// GET /api/v1/agent/version
func (h *AgentHandler) GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": readVersionFile(h.binaryDir),
	})
}

// RefreshBinaries re-scans the binary directory (for hot-reload after upload)
// POST /api/v1/agent/refresh
func (h *AgentHandler) RefreshBinaries(c *gin.Context) {
	h.refreshCache()

	h.mu.RLock()
	count := len(h.cache)
	h.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"message": "Binary cache refreshed",
		"count":   count,
	})
}

// InstallScript returns a shell script that auto-downloads and installs the agent
// GET /api/v1/agent/install.sh
func (h *AgentHandler) InstallScript(c *gin.Context) {
	// Determine the server's base URL from the request
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if fwd := c.GetHeader("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	script := fmt.Sprintf(`#!/bin/bash
set -e

# Lute Agent Installer
# Usage: curl -sSL %s/api/v1/agent/install.sh | bash -s -- --machine-id <ID> --server <GRPC_ADDR>

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
esac

INSTALL_DIR="/usr/local/bin"
BINARY_NAME="lute-agent"

echo "==> Detecting platform: ${OS}/${ARCH}"
echo "==> Downloading agent from %s ..."

curl -fSL -o "/tmp/${BINARY_NAME}" \
  "%s/api/v1/agent/download/${OS}/${ARCH}"

chmod +x "/tmp/${BINARY_NAME}"
sudo mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"

echo "==> Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
${INSTALL_DIR}/${BINARY_NAME} --version

echo ""
echo "==> Run the agent:"
echo "    ${BINARY_NAME} --server <GRPC_HOST:PORT> --machine-id <MACHINE_ID>"
`, baseURL, baseURL, baseURL)

	c.Data(http.StatusOK, "text/x-shellscript", []byte(script))
}

// RegisterFromAgent handles POST /api/v1/agent/register
// Called by the agent during --setup to create a machine + agent record
func (h *AgentHandler) RegisterFromAgent(c *gin.Context) {
	var req AgentSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ClaimCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "claim_code is required. Open the Add Machine dialog in the Lute UI (while logged in), copy the full command including --claim-code, and run it on this VM.",
		})
		return
	}

	ctx := c.Request.Context()

	// Resolve user from one-time claim code (required)
	uidStr, ok := h.consumeClaimCode(req.ClaimCode)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid or expired claim code. Codes are single-use and expire after 15 minutes. Open the Add Machine dialog in the Lute UI (while logged in), copy the full command again (it includes a new --claim-code), and run it on this VM.",
		})
		return
	}
	userID, err := primitive.ObjectIDFromHex(uidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid claim code format. Use the exact command from the Add Machine dialog in the Lute UI.",
		})
		return
	}

	// Build metadata from agent system info
	metadata := map[string]interface{}{
		"hostname": req.Hostname,
		"os":       req.OS,
		"arch":     req.Arch,
		"cpus":     req.CPUs,
		"ip":       req.IP,
	}
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	// Create the machine record (userID from claim code above)
	machine := &models.Machine{
		UserID:      userID,
		Name:        req.Name,
		Description: fmt.Sprintf("Registered from agent on %s (%s/%s)", req.Hostname, req.OS, req.Arch),
		Status:      "pending",
		Metadata:    metadata,
	}
	if err := h.machineRepo.Create(ctx, machine); err != nil {
		log.Printf("Failed to create machine: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create machine"})
		return
	}

	// Update machine with agent information
	machine.Status = "registered"
	machine.AgentIP = req.IP
	machine.AgentVersion = req.Version
	machine.LastSeen = time.Now()
	
	if err := h.machineRepo.Update(ctx, machine.ID, machine); err != nil {
		log.Printf("Failed to update machine with agent info: %v", err)
		// Clean up: delete the machine we just created
		_ = h.machineRepo.Delete(ctx, machine.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register agent"})
		return
	}

	// Derive gRPC address from the request's Host header
	// This ensures the agent connects to the same hostname it used for HTTP
	host := c.Request.Host
	if host == "" {
		// Fallback: try to extract from API URL if provided in headers
		host = c.GetHeader("Host")
		if host == "" {
			// Last resort: use config (but this might be 0.0.0.0)
			host = h.cfg.GRPC.Host
			if host == "0.0.0.0" || host == "" {
				host = "localhost"
			}
		}
	}

	// Remove port from host if present (we'll add gRPC port)
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	grpcAddr := fmt.Sprintf("%s:%s", host, h.cfg.GRPC.Port)

	log.Printf("Agent registered: machine=%s host=%s grpc=%s",
		machine.ID.Hex(), req.Hostname, grpcAddr)

	c.JSON(http.StatusCreated, AgentSetupResponse{
		MachineID:   machine.ID.Hex(),
		GRPCAddress: grpcAddr,
		Message:     "Machine registered successfully",
	})
}

// CreateClaimCode handles POST /api/v1/agent/claim-code (authenticated).
// Returns a short-lived code the user can pass to the agent so the new machine is linked to them.
func (h *AgentHandler) CreateClaimCode(c *gin.Context) {
	uid, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userIDStr, _ := uid.(string)
	code, expiresAt := h.createClaimCode(userIDStr)
	c.JSON(http.StatusOK, gin.H{
		"code":       code,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

// --- helpers ---

func (h *AgentHandler) availableKeys() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	keys := make([]string, 0, len(h.cache))
	for k := range h.cache {
		keys = append(keys, k)
	}
	return keys
}

// parseFilename extracts OS and arch from "lute-agent-<os>-<arch>[.exe]"
func parseFilename(name string) (string, string) {
	name = strings.TrimSuffix(name, ".exe")
	parts := strings.Split(name, "-")
	// expect: lute-agent-linux-amd64
	if len(parts) < 4 {
		return "", ""
	}
	return parts[2], parts[3]
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func readVersionFile(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

// ===========================================================================
// Agent management REST endpoints (used by UI to control agents)
// ===========================================================================

// SendCommandRequest is the JSON body for queueing a command
type SendCommandRequest struct {
	Command string            `json:"command" binding:"required"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// SendCommand queues a command for an agent to execute
// POST /api/v1/agent/command/:machineId
func (h *AgentHandler) SendCommand(c *gin.Context) {
	machineIDStr := c.Param("machineId")
	machineID, err := primitive.ObjectIDFromHex(machineIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid machine_id"})
		return
	}

	var req SendCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Look up the machine to get its agent ID
	machine, err := h.machineRepo.GetByID(ctx, machineID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
		return
	}

	if machine.Status == "" || machine.Status == "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "machine has no agent connected"})
		return
	}

	cmd := &models.Command{
		MachineID: machineID,
		Command:   req.Command,
		Args:      req.Args,
		Env:       req.Env,
		Status:    "pending",
	}

	if err := h.commandRepo.Create(ctx, cmd); err != nil {
		log.Printf("Failed to create command: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue command"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"command_id": cmd.ID.Hex(),
		"status":     "pending",
		"message":    "Command queued for agent",
	})
}

// ListCommands returns commands for a machine
// GET /api/v1/agent/commands/:machineId
func (h *AgentHandler) ListCommands(c *gin.Context) {
	machineIDStr := c.Param("machineId")
	machineID, err := primitive.ObjectIDFromHex(machineIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid machine_id"})
		return
	}

	ctx := c.Request.Context()
	commands, err := h.commandRepo.GetByMachineID(ctx, machineID, 50)
	if err != nil {
		log.Printf("Failed to list commands: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list commands"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"commands": commands,
		"count":    len(commands),
	})
}

// GetAgentStatus returns the status of the agent for a machine
// GET /api/v1/agent/status/:machineId
func (h *AgentHandler) GetAgentStatus(c *gin.Context) {
	machineIDStr := c.Param("machineId")
	machineID, err := primitive.ObjectIDFromHex(machineIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid machine_id"})
		return
	}

	ctx := c.Request.Context()

	machine, err := h.machineRepo.GetByID(ctx, machineID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
		return
	}

	result := gin.H{
		"machine_id":     machine.ID.Hex(),
		"machine_name":   machine.Name,
		"machine_status": machine.Status,
	}

	if machine.Status != "pending" && !machine.LastSeen.IsZero() {
		result["agent_ip"] = machine.AgentIP
		result["agent_version"] = machine.AgentVersion
		result["last_seen"] = machine.LastSeen
		result["metrics"] = machine.Metrics
	}

	c.JSON(http.StatusOK, result)
}

// GetCommandResult returns the result of a specific command
// GET /api/v1/agent/command/:commandId
func (h *AgentHandler) GetCommandResult(c *gin.Context) {
	cmdIDStr := c.Param("commandId")
	cmdID, err := primitive.ObjectIDFromHex(cmdIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid command_id"})
		return
	}

	ctx := c.Request.Context()
	cmd, err := h.commandRepo.GetByID(ctx, cmdID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "command not found"})
		return
	}

	c.JSON(http.StatusOK, cmd)
}
