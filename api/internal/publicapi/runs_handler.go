package publicapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/httpx"
	"github.com/lute/api/internal/queue"
	"github.com/lute/api/internal/runs"
)

type RunsHandler struct {
	runs *runs.Service
}

func NewRunsHandler(svc *runs.Service) *RunsHandler {
	return &RunsHandler{runs: svc}
}

func (h *RunsHandler) Create(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	var req CreateRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Webhook != nil && req.Webhook.URL == "" {
		httpx.Invalid(c, "webhook.url is required when webhook is provided", map[string]string{"webhook.url": "required"})
		return
	}

	apiKeyID, _ := id.FromHex(c.GetString("api_key_id"))
	run := &models.Run{
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
		run.WebhookSecret = req.Webhook.Secret
		if run.WebhookSecret == "" {
			generatedSecret = newWebhookSecret()
			run.WebhookSecret = generatedSecret
		}
	}

	ctx := c.Request.Context()
	run, created, err := h.runs.Enqueue(ctx, run, runs.Job{
		Payload:  req.Payload,
		Selector: req.Selector,
		Opts: queue.EnqueueOpts{
			Priority:   req.Priority,
			Delay:      time.Duration(req.DelayMs) * time.Millisecond,
			MaxRetries: req.MaxRetries,
			TimeoutSec: req.TimeoutSec,
		},
	})
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	if !created {
		c.JSON(http.StatusOK, CreateRunResponse{RunResponse: h.response(ctx, run)})
		return
	}
	c.JSON(http.StatusCreated, CreateRunResponse{RunResponse: h.response(ctx, run), WebhookSecret: generatedSecret})
}

func (h *RunsHandler) Get(c *gin.Context) {
	run, ok := h.owned(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, h.response(c.Request.Context(), run))
}

func (h *RunsHandler) List(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)

	ctx := c.Request.Context()
	list, total, err := h.runs.List(ctx, repos.RunListFilter{
		UserID: userID,
		Queue:  c.Query("queue"),
		Type:   c.Query("type"),
	}, offset, limit)
	if err != nil {
		httpx.Internal(c, err)
		return
	}

	resp := ListRunsResponse{
		Runs:   make([]RunResponse, 0, len(list)),
		Total:  total,
		Offset: offset,
		Limit:  limit,
	}
	for i := range list {
		resp.Runs = append(resp.Runs, h.response(ctx, &list[i]))
	}
	c.JSON(http.StatusOK, resp)
}

func (h *RunsHandler) Retry(c *gin.Context) {
	run, ok := h.owned(c)
	if !ok {
		return
	}
	if _, err := h.runs.Retry(c.Request.Context(), run.JobID); err != nil {
		runs.WriteError(c, err, "queue state lost for this run; create a new run")
		return
	}
	c.JSON(http.StatusOK, h.response(c.Request.Context(), run))
}

func (h *RunsHandler) Cancel(c *gin.Context) {
	run, ok := h.owned(c)
	if !ok {
		return
	}
	if err := h.runs.Cancel(c.Request.Context(), run.JobID); err != nil {
		runs.WriteError(c, err, "run has no queue state")
		return
	}
	c.JSON(http.StatusOK, h.response(c.Request.Context(), run))
}

func (h *RunsHandler) Logs(c *gin.Context) {
	run, ok := h.owned(c)
	if !ok {
		return
	}
	q, ok := runs.ReadLogQuery(c)
	if !ok {
		return
	}
	page, err := h.runs.Logs(c.Request.Context(), run.JobID, q)
	if err != nil {
		runs.WriteError(c, err, "run not found")
		return
	}
	c.JSON(http.StatusOK, page)
}

// owned loads the :id run if the caller started it, or answers 404.
func (h *RunsHandler) owned(c *gin.Context) (*models.Run, bool) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return nil, false
	}
	run, err := h.runs.Owned(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		httpx.NotFoundOrInternal(c, err, "run not found")
		return nil, false
	}
	return run, true
}

func (h *RunsHandler) response(ctx context.Context, run *models.Run) RunResponse {
	resp := RunResponse{
		ID:             run.JobID,
		Queue:          run.Queue,
		Type:           run.Type,
		Status:         "unknown",
		IdempotencyKey: run.IdempotencyKey,
		WebhookURL:     run.WebhookURL,
		WebhookEvents:  run.WebhookEvents,
	}
	if job, err := h.runs.Job(ctx, run.JobID); err == nil {
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
	if exec, err := h.runs.Execution(ctx, run.JobID); err == nil {
		resp.FinishedAt = exec.FinishedAt.UTC()
		resp.ElapsedMs = exec.ElapsedMs
		if exec.Success {
			resp.Status = "done"
		} else if resp.Status == "unknown" {
			resp.Status = "failed"
		}
	}
	if resp.EnqueuedAt.IsZero() {
		resp.EnqueuedAt = run.CreatedAt.UTC()
	}
	return resp
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
