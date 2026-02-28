package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	luteGrpc "github.com/lute/api/grpc"
	"github.com/lute/api/queue"
)

type JobHandler struct {
	engine  *queue.Engine
	stats   *queue.StatsAggregator
	grpcSrv *luteGrpc.Server
}

func NewJobHandler(engine *queue.Engine, stats *queue.StatsAggregator, grpcSrv *luteGrpc.Server) *JobHandler {
	return &JobHandler{engine: engine, stats: stats, grpcSrv: grpcSrv}
}

type EnqueueRequest struct {
	Queue      string          `json:"queue" binding:"required"`
	Type       string          `json:"type" binding:"required"`
	Payload    json.RawMessage `json:"payload"`
	Priority   float64         `json:"priority"`
	DelayMs    int64           `json:"delay_ms"`
	MaxRetries int             `json:"max_retries"`
	TimeoutSec int             `json:"timeout_sec"`
}

// Enqueue creates a new job.
// POST /api/v1/jobs
func (h *JobHandler) Enqueue(c *gin.Context) {
	var req EnqueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job := &queue.Job{
		ID:      uuid.New().String(),
		Queue:   req.Queue,
		Type:    req.Type,
		Payload: req.Payload,
	}

	opts := queue.EnqueueOpts{
		Priority:   req.Priority,
		MaxRetries: req.MaxRetries,
		TimeoutSec: req.TimeoutSec,
	}
	if req.DelayMs > 0 {
		opts.Delay = time.Duration(req.DelayMs) * time.Millisecond
	}

	ctx := c.Request.Context()
	if err := h.engine.Enqueue(ctx, job, opts); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.stats.RecordEnqueued(ctx, req.Queue)

	if h.grpcSrv != nil {
		h.grpcSrv.DispatchJob(ctx, req.Queue)
	}

	c.JSON(http.StatusCreated, gin.H{
		"job_id":  job.ID,
		"status":  job.Status,
		"message": "Job enqueued",
	})
}

// GetJob returns job details.
// GET /api/v1/jobs/:id
func (h *JobHandler) GetJob(c *gin.Context) {
	jobID := c.Param("id")
	job, err := h.engine.GetJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

// RetryJob re-enqueues a failed job.
// POST /api/v1/jobs/:id/retry
func (h *JobHandler) RetryJob(c *gin.Context) {
	jobID := c.Param("id")
	ctx := c.Request.Context()

	job, err := h.engine.GetJob(ctx, jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	job.Status = "pending"
	job.Attempts = 0
	job.Error = ""
	job.DoneAt = 0
	if err := h.engine.Enqueue(ctx, job, queue.EnqueueOpts{MaxRetries: job.MaxRetries, TimeoutSec: job.TimeoutSec}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.stats.RecordEnqueued(ctx, job.Queue)
	c.JSON(http.StatusOK, gin.H{"message": "Job re-enqueued", "job_id": job.ID})
}

// CancelJob cancels a pending job.
// DELETE /api/v1/jobs/:id
func (h *JobHandler) CancelJob(c *gin.Context) {
	jobID := c.Param("id")
	if err := h.engine.CancelJob(c.Request.Context(), jobID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Job cancelled"})
}
