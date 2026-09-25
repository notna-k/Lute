package worker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/types"
	"github.com/lute/api/internal/httpx"
)

const claimCodeExpiry = 15 * time.Minute

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

type claimEntry struct {
	UserID    string
	ExpiresAt time.Time
}

// CreateClaimCode issues a single-use code that lets a new host register as the caller.
func (h *WorkerHandler) CreateClaimCode(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	code, expiresAt, err := h.issueClaimCode(userID.Hex())
	if err != nil {
		httpx.Internal(c, err)
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

func (h *WorkerHandler) RegisterFromWorker(c *gin.Context) {
	var req WorkerSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.ClaimCode == "" {
		httpx.Error(c, http.StatusBadRequest, "claim_code is required. Open the Add Worker dialog in the Lute UI (while logged in), copy the full command including --claim-code, and run it on this host.")
		return
	}

	ctx := c.Request.Context()

	userIDStr, ok := h.consumeClaimCode(req.ClaimCode)
	if !ok {
		httpx.Error(c, http.StatusBadRequest, "invalid or expired claim code. Codes are single-use and expire after 15 minutes. Open the Add Worker dialog in the Lute UI, copy the full command again, and run it on this host.")
		return
	}

	userID, err := id.FromHex(userIDStr)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid claim code format. Use the exact command from the Add Worker dialog in the Lute UI.")
		return
	}

	agentIP := strings.TrimSpace(req.IP)
	if agentIP == "" {
		agentIP = c.ClientIP()
	}

	if agentIP != "" {
		conflictingWorker, err := h.reclaimStaleWorkersAtIP(ctx, userID, agentIP)
		if err != nil {
			httpx.Internal(c, fmt.Errorf("reclaim stale workers at %s: %w", agentIP, err))
			return
		}
		if conflictingWorker != nil {
			httpx.Error(c, http.StatusConflict, fmt.Sprintf("a worker (%q, id %s) is already registered and alive at IP %s; delete it first or stop its agent before registering a new one", conflictingWorker.Name, conflictingWorker.ID.Hex(), agentIP))
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
		httpx.Internal(c, fmt.Errorf("register worker: %w", err))
		return
	}

	grpcAddr := h.resolveGRPCAddress(c)
	slog.Info("worker registered from agent", "worker_id", worker.ID.Hex(), "host", req.Hostname, "grpc", grpcAddr)

	c.JSON(http.StatusCreated, WorkerSetupResponse{
		WorkerID:    worker.ID.Hex(),
		GRPCAddress: grpcAddr,
		Message:     "Worker registered successfully",
	})
}

// reclaimStaleWorkersAtIP returns a live worker at agentIP if there is one; otherwise it
// deletes the stale ones there and returns nil.
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
		if conn := h.connectionMgr.Get(worker.ID.Hex()); conn != nil {
			conn.Shutdown()
		}
		if err := h.workerRepo.Delete(ctx, worker.ID); err != nil {
			return nil, fmt.Errorf("delete stale worker %s: %w", worker.ID.Hex(), err)
		}
		slog.Info("reclaimed stale worker", "worker_id", worker.ID.Hex(), "name", worker.Name, "status", worker.Status, "ip", agentIP)
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
