package worker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/lute/api/internal/db/types"
	luteGrpc "github.com/lute/api/internal/grpc"
)

const claimCodeExpiry = 15 * time.Minute

type WorkerBinaryInfo struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
}

type WorkerSetupRequest struct {
	Name      string            `json:"name" binding:"required"`
	Hostname  string            `json:"hostname"`
	OS        string            `json:"os"`
	Arch      string            `json:"arch"`
	CPUs      int               `json:"cpus"`
	IP        string            `json:"ip"`
	Version   string            `json:"version"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	ClaimCode string            `json:"claim_code,omitempty"`
}

type WorkerSetupResponse struct {
	WorkerID    string `json:"worker_id"`
	GRPCAddress string `json:"grpc_address"`
	Message     string `json:"message"`
}

type SendCommandRequest struct {
	Command string            `json:"command" binding:"required"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type claimEntry struct {
	UserID    string
	ExpiresAt time.Time
}

// WorkerHandler serves worker binaries, handles registration, claim codes, and worker management.
type WorkerHandler struct {
	binaryDir     string
	binaryMu      sync.RWMutex           // guards binaryCache — SHA256 is expensive, so cache is needed
	binaryCache   map[string]*WorkerBinaryInfo
	claimMu       sync.RWMutex           // guards claimCodes
	claimCodes    map[string]*claimEntry
	cfg           *config.Config
	workerRepo    *repos.WorkerRepository
	commandRepo   *repos.CommandRepository
	workerService *WorkerService
	connectionMgr *luteGrpc.ConnectionManager
	grpcServer    *luteGrpc.Server
}

func NewWorkerHandler(
	binaryDir string,
	cfg *config.Config,
	workerRepo *repos.WorkerRepository,
	commandRepo *repos.CommandRepository,
	connectionMgr *luteGrpc.ConnectionManager,
	grpcServer *luteGrpc.Server,
) *WorkerHandler {
	handler := &WorkerHandler{
		binaryDir:     binaryDir,
		binaryCache:   make(map[string]*WorkerBinaryInfo),
		claimCodes:    make(map[string]*claimEntry),
		cfg:           cfg,
		workerRepo:    workerRepo,
		commandRepo:   commandRepo,
		workerService: NewWorkerService(workerRepo),
		connectionMgr: connectionMgr,
		grpcServer:    grpcServer,
	}
	handler.refreshBinaryCache()
	return handler
}

// ── Claim codes ───────────────────────────────────────────────────────────────

func (h *WorkerHandler) CreateClaimCode(c *gin.Context) {
	rawUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userIDStr, _ := rawUserID.(string)

	code, expiresAt, err := h.issueClaimCode(userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate claim code"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":       code,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

func (h *WorkerHandler) issueClaimCode(userID string) (string, time.Time, error) {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", time.Time{}, err
	}
	code := hex.EncodeToString(randomBytes)
	expiresAt := time.Now().Add(claimCodeExpiry)

	h.claimMu.Lock()
	defer h.claimMu.Unlock()

	for existingCode, entry := range h.claimCodes {
		if time.Now().After(entry.ExpiresAt) {
			delete(h.claimCodes, existingCode)
		}
	}
	h.claimCodes[code] = &claimEntry{UserID: userID, ExpiresAt: expiresAt}
	return code, expiresAt, nil
}

func (h *WorkerHandler) consumeClaimCode(code string) (string, bool) {
	if code == "" {
		return "", false
	}
	h.claimMu.Lock()
	defer h.claimMu.Unlock()

	entry, ok := h.claimCodes[code]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return "", false
	}
	delete(h.claimCodes, code)
	return entry.UserID, true
}

// ── Worker registration ───────────────────────────────────────────────────────

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

	userIDStr, ok := h.consumeClaimCode(req.ClaimCode)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid or expired claim code. Codes are single-use and expire after 15 minutes. Open the Add Worker dialog in the Lute UI, copy the full command again, and run it on this host.",
		})
		return
	}

	userID, err := id.FromHex(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid claim code format. Use the exact command from the Add Worker dialog in the Lute UI."})
		return
	}

	agentIP := strings.TrimSpace(req.IP)
	if agentIP == "" {
		agentIP = c.ClientIP()
	}

	if agentIP != "" {
		conflictingWorker, err := h.reclaimStaleWorkersAtIP(ctx, userID, agentIP)
		if err != nil {
			log.Printf("Failed to reclaim workers at IP %s: %v", agentIP, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reclaim stale worker at this IP"})
			return
		}
		if conflictingWorker != nil {
			c.JSON(http.StatusConflict, gin.H{
				"error": fmt.Sprintf("a worker (%q, id %s) is already registered and alive at IP %s; delete it first or stop its agent before registering a new one", conflictingWorker.Name, conflictingWorker.ID.Hex(), agentIP),
			})
			return
		}
	}

	metadata := map[string]interface{}{
		"hostname": req.Hostname,
		"os":       req.OS,
		"arch":     req.Arch,
		"cpus":     req.CPUs,
		"ip":       agentIP,
	}
	for key, value := range req.Metadata {
		metadata[key] = value
	}

	lastSeen := types.NewMilliTime(time.Now())
	worker := &models.Worker{
		UserID:       userID,
		Name:         req.Name,
		Description:  fmt.Sprintf("Registered from agent on %s (%s/%s)", req.Hostname, req.OS, req.Arch),
		Status:       "registered",
		AgentIP:      agentIP,
		AgentVersion: req.Version,
		LastSeen:     &lastSeen,
		Metadata:     metadata,
	}
	if err := h.workerRepo.Create(ctx, worker); err != nil {
		log.Printf("Failed to create worker: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register worker"})
		return
	}

	grpcAddr := h.resolveGRPCAddress(c)
	log.Printf("Worker registered: id=%s host=%s grpc=%s", worker.ID.Hex(), req.Hostname, grpcAddr)

	c.JSON(http.StatusCreated, WorkerSetupResponse{
		WorkerID:    worker.ID.Hex(),
		GRPCAddress: grpcAddr,
		Message:     "Worker registered successfully",
	})
}

// reclaimStaleWorkersAtIP returns the conflicting worker if one is alive at agentIP,
// or deletes stale (disconnected) workers and returns nil.
func (h *WorkerHandler) reclaimStaleWorkersAtIP(ctx context.Context, userID id.ID, agentIP string) (*models.Worker, error) {
	existing, err := h.workerRepo.GetByUserIDAndIP(ctx, userID, agentIP)
	if err != nil {
		return nil, fmt.Errorf("check existing workers by IP: %w", err)
	}

	for _, worker := range existing {
		if worker.Status == "alive" || worker.Status == "registered" {
			return worker, nil
		}
	}

	for _, worker := range existing {
		if h.connectionMgr != nil {
			if conn := h.connectionMgr.Get(worker.ID.Hex()); conn != nil {
				conn.Shutdown()
			}
		}
		if err := h.workerRepo.Delete(ctx, worker.ID); err != nil {
			return nil, fmt.Errorf("delete stale worker %s: %w", worker.ID.Hex(), err)
		}
		log.Printf("Reclaimed stale worker id=%s name=%q status=%s on IP %s", worker.ID.Hex(), worker.Name, worker.Status, agentIP)
	}
	return nil, nil
}

func (h *WorkerHandler) resolveGRPCAddress(c *gin.Context) string {
	host := c.Request.Host
	if host == "" {
		host = c.GetHeader("Host")
	}
	if host == "" {
		host = h.cfg.GRPC.Host
		if host == "0.0.0.0" || host == "" {
			host = "localhost"
		}
	}
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}
	return fmt.Sprintf("%s:%s", host, h.cfg.GRPC.Port)
}

// ── Worker binary serving ─────────────────────────────────────────────────────

func (h *WorkerHandler) ListBinaries(c *gin.Context) {
	h.binaryMu.RLock()
	defer h.binaryMu.RUnlock()

	binaries := make([]*WorkerBinaryInfo, 0, len(h.binaryCache))
	for _, binaryInfo := range h.binaryCache {
		binaries = append(binaries, binaryInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"binaries": binaries,
		"version":  readVersionFile(h.binaryDir),
	})
}

func (h *WorkerHandler) DownloadBinary(c *gin.Context) {
	h.serveBinary(c, c.Param("os"), c.Param("arch"))
}

func (h *WorkerHandler) DownloadAutoDetect(c *gin.Context) {
	h.serveBinary(c, c.DefaultQuery("os", "linux"), c.DefaultQuery("arch", "amd64"))
}

func (h *WorkerHandler) serveBinary(c *gin.Context, osName, arch string) {
	cacheKey := osName + "/" + arch

	h.binaryMu.RLock()
	binaryInfo, ok := h.binaryCache[cacheKey]
	availableKeys := make([]string, 0, len(h.binaryCache))
	if !ok {
		for key := range h.binaryCache {
			availableKeys = append(availableKeys, key)
		}
	}
	h.binaryMu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error":     fmt.Sprintf("no worker binary for %s/%s", osName, arch),
			"available": availableKeys,
		})
		return
	}

	fullPath := filepath.Join(h.binaryDir, binaryInfo.Filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", binaryInfo.Filename))
	c.Header("X-Worker-Version", binaryInfo.Version)
	c.Header("X-Worker-SHA256", binaryInfo.SHA256)
	c.File(fullPath)
}

func (h *WorkerHandler) GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"version": readVersionFile(h.binaryDir)})
}

func (h *WorkerHandler) RefreshBinaries(c *gin.Context) {
	h.refreshBinaryCache()

	h.binaryMu.RLock()
	count := len(h.binaryCache)
	h.binaryMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{"message": "Binary cache refreshed", "count": count})
}

func (h *WorkerHandler) refreshBinaryCache() {
	h.binaryMu.Lock()
	defer h.binaryMu.Unlock()

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
		filename := entry.Name()
		if !strings.HasPrefix(filename, "lute-worker-") {
			continue
		}

		osName, arch := parseWorkerFilename(filename)
		if osName == "" || arch == "" {
			continue
		}

		fullPath := filepath.Join(h.binaryDir, filename)
		fileInfo, err := entry.Info()
		if err != nil {
			continue
		}

		checksum, err := sha256File(fullPath)
		if err != nil {
			log.Printf("Warning: cannot compute checksum for %s: %v", filename, err)
			continue
		}

		cacheKey := osName + "/" + arch
		newCache[cacheKey] = &WorkerBinaryInfo{
			OS:       osName,
			Arch:     arch,
			Version:  readVersionFile(h.binaryDir),
			Filename: filename,
			SHA256:   checksum,
			Size:     fileInfo.Size(),
		}
		log.Printf("Indexed worker binary: %s (%s/%s, %d bytes)", filename, osName, arch, fileInfo.Size())
	}

	h.binaryCache = newCache
}

func (h *WorkerHandler) InstallScript(c *gin.Context) {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if forwarded := c.GetHeader("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
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

// ── Worker management ─────────────────────────────────────────────────────────

func (h *WorkerHandler) SendCommand(c *gin.Context) {
	workerID, err := id.FromHex(c.Param("id"))
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

	worker, err := h.workerRepo.GetByID(ctx, workerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "worker not found"})
		return
	}
	if worker.Status == "" || worker.Status == "pending" {
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
	workerID, err := id.FromHex(c.Param("id"))
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

	c.JSON(http.StatusOK, gin.H{"commands": commands, "count": len(commands)})
}

func (h *WorkerHandler) GetWorkerLiveStatus(c *gin.Context) {
	workerID, err := id.FromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid worker_id"})
		return
	}

	ctx := c.Request.Context()
	worker, err := h.workerRepo.GetByID(ctx, workerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "worker not found"})
		return
	}

	result := gin.H{
		"worker_id": worker.ID.Hex(),
		"name":      worker.Name,
		"status":    worker.Status,
	}
	if worker.Status != "pending" && !worker.LastSeen.IsZero() {
		result["agent_ip"] = worker.AgentIP
		result["agent_version"] = worker.AgentVersion
		result["last_seen"] = worker.LastSeen
		result["metrics"] = worker.Metrics
	}

	c.JSON(http.StatusOK, result)
}

func (h *WorkerHandler) GetCommandResult(c *gin.Context) {
	cmdID, err := id.FromHex(c.Param("commandId"))
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

// ── File utilities ────────────────────────────────────────────────────────────

func parseWorkerFilename(filename string) (osName, arch string) {
	filename = strings.TrimSuffix(filename, ".exe")
	parts := strings.Split(filename, "-")
	if len(parts) < 4 {
		return "", ""
	}
	return parts[2], parts[3]
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func readVersionFile(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}
