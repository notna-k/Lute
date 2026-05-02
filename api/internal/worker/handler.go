package worker

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

	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	luteGrpc "github.com/lute/api/internal/grpc"
)

// WorkerBinaryInfo describes one compiled worker binary
type WorkerBinaryInfo struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
}

// WorkerSetupRequest is sent by the worker during --setup to register a new machine
type WorkerSetupRequest struct {
	Name      string            `json:"name" binding:"required"`
	Hostname  string            `json:"hostname"`
	OS        string            `json:"os"`
	Arch      string            `json:"arch"`
	CPUs      int               `json:"cpus"`
	IP        string            `json:"ip"`
	Version   string            `json:"version"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	ClaimCode string            `json:"claim_code,omitempty"` // optional; links machine to user when valid
}

// WorkerSetupResponse is returned after bootstrap registration creates a worker row.
type WorkerSetupResponse struct {
	WorkerID    string `json:"worker_id"`
	GRPCAddress string `json:"grpc_address"`
	Message     string `json:"message"`
}

// claimEntry holds a short-lived claim code that links a new machine to a user
type claimEntry struct {
	UserID    string
	ExpiresAt time.Time
}

// WorkerHandler serves worker binaries, bootstrap, CRUD, commands, and connection info.
type WorkerHandler struct {
	binaryDir     string
	mu            sync.RWMutex
	cache         map[string]*WorkerBinaryInfo
	claimMu       sync.RWMutex
	claimCodes    map[string]*claimEntry
	cfg           *config.Config
	workerRepo    *repos.WorkerRepository
	commandRepo   *repos.CommandRepository
	workerService *WorkerService
	connMgr       *luteGrpc.ConnectionManager
}

// NewWorkerHandler creates a handler that serves worker binaries from binaryDir.
func NewWorkerHandler(
	binaryDir string,
	cfg *config.Config,
	workerRepo *repos.WorkerRepository,
	commandRepo *repos.CommandRepository,
	connMgr *luteGrpc.ConnectionManager,
) *WorkerHandler {
	h := &WorkerHandler{
		binaryDir:     binaryDir,
		cache:         make(map[string]*WorkerBinaryInfo),
		claimCodes:    make(map[string]*claimEntry),
		cfg:           cfg,
		workerRepo:    workerRepo,
		commandRepo:   commandRepo,
		workerService: NewWorkerService(workerRepo),
		connMgr:       connMgr,
	}
	h.refreshCache()
	return h
}

const claimCodeLen = 20
const claimCodeExpiry = 15 * time.Minute
const claimCodeChars = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ"

func (h *WorkerHandler) createClaimCode(userID string) (code string, expiresAt time.Time) {
	b := make([]byte, claimCodeLen)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = claimCodeChars[int(b[i])%len(claimCodeChars)]
	}
	code = string(b)
	expiresAt = time.Now().Add(claimCodeExpiry)
	h.claimMu.Lock()
	defer h.claimMu.Unlock()
	for k, v := range h.claimCodes {
		if time.Now().After(v.ExpiresAt) {
			delete(h.claimCodes, k)
		}
	}
	h.claimCodes[code] = &claimEntry{UserID: userID, ExpiresAt: expiresAt}
	return code, expiresAt
}

func (h *WorkerHandler) consumeClaimCode(code string) (userID string, ok bool) {
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

func (h *WorkerHandler) refreshCache() {
	h.mu.Lock()
	defer h.mu.Unlock()

	entries, err := os.ReadDir(h.binaryDir)
	if err != nil {
		log.Printf("Warning: cannot read worker binary dir %s: %v", h.binaryDir, err)
		return
	}

	newCache := make(map[string]*WorkerBinaryInfo)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "lute-worker-") {
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
		newCache[key] = &WorkerBinaryInfo{
			OS:       osName,
			Arch:     arch,
			Version:  readVersionFile(h.binaryDir),
			Filename: name,
			SHA256:   checksum,
			Size:     info.Size(),
		}
		log.Printf("Indexed worker binary: %s (%s/%s, %d bytes)", name, osName, arch, info.Size())
	}

	h.cache = newCache
}

func (h *WorkerHandler) ListBinaries(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	binaries := make([]*WorkerBinaryInfo, 0, len(h.cache))
	for _, b := range h.cache {
		binaries = append(binaries, b)
	}

	c.JSON(http.StatusOK, gin.H{
		"binaries": binaries,
		"version":  readVersionFile(h.binaryDir),
	})
}

func (h *WorkerHandler) DownloadBinary(c *gin.Context) {
	osName := c.Param("os")
	arch := c.Param("arch")
	key := osName + "/" + arch

	h.mu.RLock()
	info, ok := h.cache[key]
	h.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     fmt.Sprintf("no worker binary for %s/%s", osName, arch),
			"available": h.availableKeys(),
		})
		return
	}

	fullPath := filepath.Join(h.binaryDir, info.Filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", info.Filename))
	c.Header("X-Worker-Version", info.Version)
	c.Header("X-Worker-SHA256", info.SHA256)
	c.File(fullPath)
}

func (h *WorkerHandler) DownloadAutoDetect(c *gin.Context) {
	osName := c.DefaultQuery("os", "linux")
	arch := c.DefaultQuery("arch", "amd64")
	key := osName + "/" + arch

	h.mu.RLock()
	info, ok := h.cache[key]
	h.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     fmt.Sprintf("no worker binary for %s/%s", osName, arch),
			"available": h.availableKeys(),
		})
		return
	}

	fullPath := filepath.Join(h.binaryDir, info.Filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", info.Filename))
	c.Header("X-Worker-Version", info.Version)
	c.Header("X-Worker-SHA256", info.SHA256)
	c.File(fullPath)
}

func (h *WorkerHandler) GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": readVersionFile(h.binaryDir),
	})
}

func (h *WorkerHandler) RefreshBinaries(c *gin.Context) {
	h.refreshCache()

	h.mu.RLock()
	count := len(h.cache)
	h.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"message": "Binary cache refreshed",
		"count":   count,
	})
}

func (h *WorkerHandler) InstallScript(c *gin.Context) {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if fwd := c.GetHeader("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)

	bootstrapPath := "/api/public/v1/workers/bootstrap"
	if strings.HasSuffix(c.Request.URL.Path, "/install.sh") {
		bootstrapPath = strings.TrimSuffix(c.Request.URL.Path, "/install.sh")
	}
	installURL := baseURL + bootstrapPath + "/install.sh"
	downloadURL := baseURL + bootstrapPath + "/download/${OS}/${ARCH}"

	script := fmt.Sprintf(`#!/bin/bash
set -e

# Lute Worker Installer
# Usage: curl -sSL %s | bash -s -- --worker-id <ID> --server <GRPC_ADDR>

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
esac

INSTALL_DIR="/usr/local/bin"
BINARY_NAME="lute-worker"

echo "==> Detecting platform: ${OS}/${ARCH}"
echo "==> Downloading worker from %s ..."

curl -fSL -o "/tmp/${BINARY_NAME}" \
  "%s"

chmod +x "/tmp/${BINARY_NAME}"
sudo mv "/tmp/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"

echo "==> Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
${INSTALL_DIR}/${BINARY_NAME} --version

echo ""
echo "==> Next step: register this host with the Lute server"
echo "    ${BINARY_NAME} setup --claim-code <CODE>"
echo ""
echo "    (copy the full command from the Add Worker dialog in the Lute UI)"
`, installURL, baseURL, downloadURL)

	c.Data(http.StatusOK, "text/x-shellscript", []byte(script))
}

func (h *WorkerHandler) RegisterFromWorker(c *gin.Context) {
	var req WorkerSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ClaimCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "claim_code is required. Open the Add Worker dialog in the Lute UI (while logged in), copy the full command including --claim-code, and run it on this host.",
		})
		return
	}

	ctx := c.Request.Context()

	uidStr, ok := h.consumeClaimCode(req.ClaimCode)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid or expired claim code. Codes are single-use and expire after 15 minutes. Open the Add Worker dialog in the Lute UI (while logged in), copy the full command again (it includes a new --claim-code), and run it on this host.",
		})
		return
	}
	userID, err := id.FromHex(uidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid claim code format. Use the exact command from the Add Worker dialog in the Lute UI.",
		})
		return
	}

	ip := strings.TrimSpace(req.IP)
	if ip == "" {
		ip = c.ClientIP()
	}

	if ip != "" {
		existing, err := h.workerRepo.GetByUserIDAndIP(ctx, userID, ip)
		if err != nil {
			log.Printf("Failed to check existing workers by IP: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register worker"})
			return
		}
		for _, w := range existing {
			if w.Status == "alive" || w.Status == "registered" {
				c.JSON(http.StatusConflict, gin.H{
					"error": fmt.Sprintf("a worker (%q, id %s) is already registered and alive at IP %s; delete it first or stop its agent before registering a new one", w.Name, w.ID.Hex(), ip),
				})
				return
			}
		}
		for _, w := range existing {
			if h.connMgr != nil {
				if conn := h.connMgr.Get(w.ID.Hex()); conn != nil {
					conn.Shutdown()
				}
			}
			if err := h.workerRepo.Delete(ctx, w.ID); err != nil {
				log.Printf("Failed to delete stale worker %s on IP %s: %v", w.ID.Hex(), ip, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reclaim stale worker at this IP"})
				return
			}
			log.Printf("Reclaimed stale worker id=%s name=%q status=%s on IP %s", w.ID.Hex(), w.Name, w.Status, ip)
		}
	}

	metadata := map[string]interface{}{
		"hostname": req.Hostname,
		"os":       req.OS,
		"arch":     req.Arch,
		"cpus":     req.CPUs,
		"ip":       ip,
	}
	for k, v := range req.Metadata {
		metadata[k] = v
	}

	w := &models.Worker{
		UserID:      userID,
		Name:        req.Name,
		Description: fmt.Sprintf("Registered from agent on %s (%s/%s)", req.Hostname, req.OS, req.Arch),
		Status:      "pending",
		Metadata:    metadata,
	}
	if err := h.workerRepo.Create(ctx, w); err != nil {
		log.Printf("Failed to create worker: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create worker"})
		return
	}

	w.Status = "registered"
	w.AgentIP = ip
	w.AgentVersion = req.Version
	w.LastSeen = time.Now()

	if err := h.workerRepo.Update(ctx, w.ID, w); err != nil {
		log.Printf("Failed to update worker with agent info: %v", err)
		_ = h.workerRepo.Delete(ctx, w.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register worker"})
		return
	}

	host := c.Request.Host
	if host == "" {
		host = c.GetHeader("Host")
		if host == "" {
			host = h.cfg.GRPC.Host
			if host == "0.0.0.0" || host == "" {
				host = "localhost"
			}
		}
	}

	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	grpcAddr := fmt.Sprintf("%s:%s", host, h.cfg.GRPC.Port)

	log.Printf("Worker registered: id=%s host=%s grpc=%s",
		w.ID.Hex(), req.Hostname, grpcAddr)

	c.JSON(http.StatusCreated, WorkerSetupResponse{
		WorkerID:    w.ID.Hex(),
		GRPCAddress: grpcAddr,
		Message:     "Worker registered successfully",
	})
}

func (h *WorkerHandler) CreateClaimCode(c *gin.Context) {
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

func (h *WorkerHandler) availableKeys() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	keys := make([]string, 0, len(h.cache))
	for k := range h.cache {
		keys = append(keys, k)
	}
	return keys
}

func parseFilename(name string) (string, string) {
	name = strings.TrimSuffix(name, ".exe")
	parts := strings.Split(name, "-")
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
	defer func() { _ = f.Close() }()

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

// SendCommandRequest is the JSON body for queueing a command
type SendCommandRequest struct {
	Command string            `json:"command" binding:"required"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

func (h *WorkerHandler) SendCommand(c *gin.Context) {
	workerIDStr := c.Param("id")
	workerID, err := id.FromHex(workerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid worker_id"})
		return
	}

	var req SendCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	w, err := h.workerRepo.GetByID(ctx, workerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "worker not found"})
		return
	}

	if w.Status == "" || w.Status == "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "worker has no active agent connection"})
		return
	}

	cmd := &models.Command{
		WorkerID: workerID,
		Command:  req.Command,
		Args:     req.Args,
		Env:      req.Env,
		Status:   "pending",
	}

	if err := h.commandRepo.Create(ctx, cmd); err != nil {
		log.Printf("Failed to create command: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue command"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"command_id": cmd.ID.Hex(),
		"status":     "pending",
		"message":    "Command queued for worker",
	})
}

func (h *WorkerHandler) ListCommands(c *gin.Context) {
	workerIDStr := c.Param("id")
	workerID, err := id.FromHex(workerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid worker_id"})
		return
	}

	ctx := c.Request.Context()
	commands, err := h.commandRepo.GetByWorkerID(ctx, workerID, 50)
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

func (h *WorkerHandler) GetWorkerLiveStatus(c *gin.Context) {
	workerIDStr := c.Param("id")
	workerOID, err := id.FromHex(workerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid worker_id"})
		return
	}

	ctx := c.Request.Context()

	w, err := h.workerRepo.GetByID(ctx, workerOID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "worker not found"})
		return
	}

	result := gin.H{
		"worker_id": w.ID.Hex(),
		"name":      w.Name,
		"status":    w.Status,
	}

	if w.Status != "pending" && !w.LastSeen.IsZero() {
		result["agent_ip"] = w.AgentIP
		result["agent_version"] = w.AgentVersion
		result["last_seen"] = w.LastSeen
		result["metrics"] = w.Metrics
	}

	c.JSON(http.StatusOK, result)
}

func (h *WorkerHandler) GetCommandResult(c *gin.Context) {
	cmdIDStr := c.Param("commandId")
	cmdID, err := id.FromHex(cmdIDStr)
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
