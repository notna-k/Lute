package publicapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	pb "github.com/lute/proto"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/queue"
)

// RunsHandler exposes user-scoped run operations. All methods require an API key
// middleware that set the "user_id" context value.
type RunsHandler struct {
	engine      *queue.Engine
	stats       *queue.StatsAggregator
	grpcSrv     *grpc.Server
	runs        *repos.RunRepository
	executions  *repos.JobExecutionRepository
}

func NewRunsHandler(
	engine *queue.Engine,
	stats *queue.StatsAggregator,
	grpcSrv *grpc.Server,
	runs *repos.RunRepository,
	executions *repos.JobExecutionRepository,
) *RunsHandler {
	return &RunsHandler{engine: engine, stats: stats, grpcSrv: grpcSrv, runs: runs, executions: executions}
}

func (h *RunsHandler) Create(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	var apiKeyID id.ID
	if v, exists := c.Get("api_key_id"); exists {
		if ak, err := id.FromHex(v.(string)); err == nil {
			apiKeyID = ak
		}
	}

	var req CreateRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if req.Webhook != nil && req.Webhook.URL == "" {
		writeError(c, http.StatusBadRequest, "invalid_request", "webhook.url is required when webhook is provided")
		return
	}

	ctx := c.Request.Context()

	if req.IdempotencyKey != "" {
		existing, err := h.runs.GetByIdempotency(ctx, userID, req.IdempotencyKey)
		if err == nil {
			resp := h.buildRunResponse(ctx, existing)
			c.JSON(http.StatusOK, CreateRunResponse{RunResponse: resp})
			return
		} else if !errors.Is(err, repos.ErrNotFound) {
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
	}

	run := &models.Run{
		JobID:          uuid.New().String(),
		UserID:         userID,
		APIKeyID:       apiKeyID,
		Queue:          req.Queue,
		Type:           req.Type,
		IdempotencyKey: req.IdempotencyKey,
	}
	var generatedSecret string
	if req.Webhook != nil {
		run.WebhookURL = req.Webhook.URL
		run.WebhookEvents = normalizeEvents(req.Webhook.Events)
		if req.Webhook.Secret != "" {
			run.WebhookSecret = req.Webhook.Secret
		} else {
			generatedSecret = newWebhookSecret()
			run.WebhookSecret = generatedSecret
		}
	}

	if err := h.runs.Create(ctx, run); err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	job := &queue.Job{
		ID:      run.JobID,
		Queue:   req.Queue,
		Type:    req.Type,
		Payload: req.Payload,
		Meta:    map[string]string{"user_id": userID.Hex(), "run_id": run.ID.Hex()},
	}
	opts := queue.EnqueueOpts{
		Priority:   req.Priority,
		MaxRetries: req.MaxRetries,
		TimeoutSec: req.TimeoutSec,
	}
	if req.DelayMs > 0 {
		opts.Delay = time.Duration(req.DelayMs) * time.Millisecond
	}
	if err := h.engine.Enqueue(ctx, job, opts); err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.stats.RecordEnqueued(ctx, req.Queue)
	if h.grpcSrv != nil {
		h.grpcSrv.DispatchQueue(ctx, req.Queue)
	}

	resp := CreateRunResponse{RunResponse: h.buildRunResponse(ctx, run)}
	if generatedSecret != "" {
		resp.WebhookSecret = generatedSecret
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *RunsHandler) Get(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	run, status := h.loadOwnedRun(c, userID)
	if status != 0 {
		return
	}
	c.JSON(http.StatusOK, h.buildRunResponse(c.Request.Context(), run))
}

func (h *RunsHandler) List(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)

	runs, total, err := h.runs.List(c.Request.Context(), repos.RunListFilter{
		UserID: userID,
		Queue:  c.Query("queue"),
		Type:   c.Query("type"),
	}, offset, limit)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	resp := ListRunsResponse{
		Runs:   make([]RunResponse, 0, len(runs)),
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}
	ctx := c.Request.Context()
	for i := range runs {
		resp.Runs = append(resp.Runs, h.buildRunResponse(ctx, &runs[i]))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *RunsHandler) Retry(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	run, status := h.loadOwnedRun(c, userID)
	if status != 0 {
		return
	}

	ctx := c.Request.Context()
	job, err := h.engine.GetJob(ctx, run.JobID)
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "queue state lost for this run; create a new run")
		return
	}
	job.Status = "pending"
	job.Attempts = 0
	job.Error = ""
	job.DoneAt = 0
	job.WorkerID = ""
	if err := h.engine.Enqueue(ctx, job, queue.EnqueueOpts{MaxRetries: job.MaxRetries, TimeoutSec: job.TimeoutSec}); err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.stats.RecordEnqueued(ctx, job.Queue)
	if h.grpcSrv != nil {
		h.grpcSrv.DispatchQueue(ctx, job.Queue)
	}
	c.JSON(http.StatusOK, h.buildRunResponse(ctx, run))
}

func (h *RunsHandler) Cancel(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	run, status := h.loadOwnedRun(c, userID)
	if status != 0 {
		return
	}
	if err := h.engine.CancelJob(c.Request.Context(), run.JobID); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_state", err.Error())
		return
	}
	c.JSON(http.StatusOK, h.buildRunResponse(c.Request.Context(), run))
}

func (h *RunsHandler) Logs(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	run, status := h.loadOwnedRun(c, userID)
	if status != 0 {
		return
	}

	ctx := c.Request.Context()
	job, err := h.engine.GetJob(ctx, run.JobID)
	if err != nil {
		writeError(c, http.StatusNotFound, "not_found", "run has no active queue state")
		return
	}

	workerID := job.WorkerID
	if workerID == "" && h.executions != nil {
		if exec, err := h.executions.GetByJobID(ctx, run.JobID); err == nil {
			workerID = exec.WorkerID
		}
	}
	if workerID == "" {
		writeError(c, http.StatusNotFound, "not_found", "no worker has executed this run yet")
		return
	}
	if h.grpcSrv == nil {
		writeError(c, http.StatusServiceUnavailable, "unavailable", "worker gateway unavailable")
		return
	}

	direction := c.DefaultQuery("direction", "tail")
	var dir pb.LogReadDirection
	switch direction {
	case "head":
		dir = pb.LogReadDirection_LOG_READ_HEAD
	case "tail":
		dir = pb.LogReadDirection_LOG_READ_TAIL
	default:
		writeError(c, http.StatusBadRequest, "invalid_request", "direction must be tail or head")
		return
	}
	limit := 200
	if ls := c.Query("limit"); ls != "" {
		if n, err := strconv.Atoi(ls); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	rpcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := h.grpcSrv.RequestJobLog(rpcCtx, workerID, &pb.JobLogRequest{
		JobId:     run.JobID,
		Direction: dir,
		Limit:     int32(limit),
	})
	if err != nil {
		if errors.Is(err, grpc.ErrNoConnection) {
			writeError(c, http.StatusServiceUnavailable, "unavailable", "worker not connected")
			return
		}
		writeError(c, http.StatusBadGateway, "bad_gateway", err.Error())
		return
	}
	out := gin.H{
		"lines":     resp.GetLines(),
		"direction": direction,
		"has_more":  resp.HasMore,
		"file_size": resp.FileSize,
	}
	c.JSON(http.StatusOK, out)
}

func (h *RunsHandler) loadOwnedRun(c *gin.Context, userID id.ID) (*models.Run, int) {
	jobID := c.Param("id")
	run, err := h.runs.GetByJobID(c.Request.Context(), jobID)
	if err != nil {
		if errors.Is(err, repos.ErrNotFound) {
			writeError(c, http.StatusNotFound, "not_found", "run not found")
			return nil, http.StatusNotFound
		}
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return nil, http.StatusInternalServerError
	}
	if run.UserID != userID {
		writeError(c, http.StatusNotFound, "not_found", "run not found")
		return nil, http.StatusNotFound
	}
	return run, 0
}

func (h *RunsHandler) buildRunResponse(ctx context.Context, run *models.Run) RunResponse {
	resp := RunResponse{
		ID:             run.JobID,
		Queue:          run.Queue,
		Type:           run.Type,
		Status:         "unknown",
		IdempotencyKey: run.IdempotencyKey,
		WebhookURL:     run.WebhookURL,
		WebhookEvents:  run.WebhookEvents,
	}
	if job, err := h.engine.GetJob(ctx, run.JobID); err == nil {
		resp.Status = job.Status
		resp.Attempts = job.Attempts
		resp.MaxRetries = job.MaxRetries
		resp.TimeoutSec = job.TimeoutSec
		resp.Error = job.Error
		resp.WorkerID = job.WorkerID
		if job.EnqueuedAt > 0 {
			resp.EnqueuedAt = time.Unix(job.EnqueuedAt, 0).UTC()
		}
		if job.StartedAt > 0 {
			resp.StartedAt = time.Unix(job.StartedAt, 0).UTC()
		}
	}
	if h.executions != nil {
		if exec, err := h.executions.GetByJobID(ctx, run.JobID); err == nil {
			resp.FinishedAt = exec.FinishedAt.UTC()
			resp.ElapsedMs = exec.ElapsedMs
			if exec.Success {
				resp.Status = "done"
			} else if resp.Status == "unknown" {
				resp.Status = "failed"
			}
		}
	}
	if resp.EnqueuedAt.IsZero() {
		resp.EnqueuedAt = run.CreatedAt.UTC()
	}
	return resp
}

func requireUserID(c *gin.Context) (id.ID, bool) {
	raw, exists := c.Get("user_id")
	if !exists {
		writeError(c, http.StatusUnauthorized, "unauthorized", "authentication required")
		return id.ID(""), false
	}
	uid, err := id.FromHex(raw.(string))
	if err != nil {
		writeError(c, http.StatusUnauthorized, "unauthorized", "invalid user context")
		return id.ID(""), false
	}
	return uid, true
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func normalizeEvents(events []string) []string {
	allowed := map[string]bool{
		"run.completed": true,
		"run.failed":    true,
		"run.started":   true,
	}
	if len(events) == 0 {
		return []string{"run.completed", "run.failed"}
	}
	seen := make(map[string]bool, len(events))
	out := make([]string, 0, len(events))
	for _, e := range events {
		if !allowed[e] || seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	if len(out) == 0 {
		return []string{"run.completed", "run.failed"}
	}
	return out
}

func newWebhookSecret() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return "whsec_" + hex.EncodeToString(buf)
}
