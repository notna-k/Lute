package jobs

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/httpx"
	"github.com/lute/api/internal/queue"
	"github.com/lute/api/internal/runs"
)

// JobHandler is the panel's raw view of queue jobs, addressed by job id.
type JobHandler struct {
	runs *runs.Service
}

func NewJobHandler(svc *runs.Service) *JobHandler {
	return &JobHandler{runs: svc}
}

type EnqueueRequest struct {
	Queue      string            `json:"queue" binding:"required"`
	Type       string            `json:"type" binding:"required"`
	Payload    json.RawMessage   `json:"payload"`
	Priority   float64           `json:"priority"`
	DelayMs    int64             `json:"delay_ms"`
	MaxRetries int               `json:"max_retries"`
	TimeoutSec int               `json:"timeout_sec"`
	Selector   map[string]string `json:"selector,omitempty"`
}

func (h *JobHandler) Enqueue(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	var req EnqueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	run, _, err := h.runs.Enqueue(c.Request.Context(),
		&models.Run{UserID: userID, Queue: req.Queue, Type: req.Type},
		runs.Job{
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
	c.JSON(http.StatusCreated, gin.H{
		"job_id":  run.JobID,
		"status":  enums.QueueJobPending,
		"message": "Job enqueued",
	})
}

func (h *JobHandler) GetJob(c *gin.Context) {
	job, err := h.runs.Job(c.Request.Context(), c.Param("id"))
	if err != nil {
		runs.WriteError(c, err, "job not found")
		return
	}
	c.JSON(http.StatusOK, job)
}

func (h *JobHandler) RetryJob(c *gin.Context) {
	job, err := h.runs.Retry(c.Request.Context(), c.Param("id"))
	if err != nil {
		runs.WriteError(c, err, "job not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Job re-enqueued", "job_id": job.ID})
}

func (h *JobHandler) CancelJob(c *gin.Context) {
	if err := h.runs.Cancel(c.Request.Context(), c.Param("id")); err != nil {
		runs.WriteError(c, err, "job not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Job cancelled"})
}

// GetJobLogs proxies a page of the job log from the worker that holds it.
func (h *JobHandler) GetJobLogs(c *gin.Context) {
	q, ok := runs.ReadLogQuery(c)
	if !ok {
		return
	}
	page, err := h.runs.Logs(c.Request.Context(), c.Param("id"), q)
	if err != nil {
		runs.WriteError(c, err, "job not found")
		return
	}
	c.JSON(http.StatusOK, page)
}
