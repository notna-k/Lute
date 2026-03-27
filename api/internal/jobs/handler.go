package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"

	pb "github.com/lute/proto"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/queue"
)

type JobHandler struct {
	engine       *queue.Engine
	stats        *queue.StatsAggregator
	grpcSrv      *grpc.Server
	jobExecRepo  *repos.JobExecutionRepository
}

func NewJobHandler(engine *queue.Engine, stats *queue.StatsAggregator, grpcSrv *grpc.Server, jobExecRepo *repos.JobExecutionRepository) *JobHandler {
	return &JobHandler{engine: engine, stats: stats, grpcSrv: grpcSrv, jobExecRepo: jobExecRepo}
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
		h.grpcSrv.DispatchQueue(ctx, req.Queue)
	}

	c.JSON(http.StatusCreated, gin.H{
		"job_id":  job.ID,
		"status":  job.Status,
		"message": "Job enqueued",
	})
}

func (h *JobHandler) GetJob(c *gin.Context) {
	jobID := c.Param("id")
	job, err := h.engine.GetJob(c.Request.Context(), jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

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
	job.WorkerID = ""
	if err := h.engine.Enqueue(ctx, job, queue.EnqueueOpts{MaxRetries: job.MaxRetries, TimeoutSec: job.TimeoutSec}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.stats.RecordEnqueued(ctx, job.Queue)
	if h.grpcSrv != nil {
		h.grpcSrv.DispatchQueue(ctx, job.Queue)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Job re-enqueued", "job_id": job.ID})
}

func (h *JobHandler) CancelJob(c *gin.Context) {
	jobID := c.Param("id")
	if err := h.engine.CancelJob(c.Request.Context(), jobID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Job cancelled"})
}

const (
	defaultLogLimit = 200
	maxLogLimit     = 500
	logRPCDeadline  = 30 * time.Second
)

// GetJobLogs proxies a chunked read of the job log file from the worker that ran (or runs) the job.
func (h *JobHandler) GetJobLogs(c *gin.Context) {
	jobID := c.Param("id")
	ctx := c.Request.Context()

	job, err := h.engine.GetJob(ctx, jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	machineID, err := h.resolveLogMachine(ctx, job)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	directionStr := c.DefaultQuery("direction", "tail")
	var dir pb.LogReadDirection
	switch directionStr {
	case "head":
		dir = pb.LogReadDirection_LOG_READ_HEAD
	case "tail":
		dir = pb.LogReadDirection_LOG_READ_TAIL
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "direction must be tail or head"})
		return
	}

	limit := defaultLogLimit
	if ls := c.Query("limit"); ls != "" {
		n, err := strconv.Atoi(ls)
		if err != nil || n < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		if n > maxLogLimit {
			n = maxLogLimit
		}
		limit = n
	}

	var anchor int64
	if cur := c.Query("cursor"); cur != "" {
		anchor, err = strconv.ParseInt(cur, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid cursor"})
			return
		}
	}

	if h.grpcSrv == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "worker gateway unavailable"})
		return
	}

	rpcCtx, cancel := context.WithTimeout(ctx, logRPCDeadline)
	defer cancel()

	pbReq := &pb.JobLogRequest{
		JobId:        jobID,
		Direction:    dir,
		Limit:        int32(limit),
		AnchorOffset: anchor,
	}

	resp, err := h.grpcSrv.RequestJobLog(rpcCtx, machineID, pbReq)
	if err != nil {
		if errors.Is(err, grpc.ErrNoConnection) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "worker not connected"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	lines := resp.GetLines()
	if lines == nil {
		lines = []string{}
	}
	out := gin.H{
		"lines":     lines,
		"direction": directionStr,
		"has_more":  resp.HasMore,
		"file_size": resp.FileSize,
	}
	if resp.NextAnchor != 0 || resp.HasMore {
		out["next_cursor"] = strconv.FormatInt(resp.NextAnchor, 10)
	}
	if resp.Error != "" {
		out["error"] = resp.Error
	}
	c.JSON(http.StatusOK, out)
}

func (h *JobHandler) resolveLogMachine(ctx context.Context, job *queue.Job) (string, error) {
	if job.Status == "running" && job.WorkerID != "" {
		return job.WorkerID, nil
	}

	if h.jobExecRepo == nil {
		return "", errors.New("no execution record available for this job")
	}

	exec, err := h.jobExecRepo.GetByJobID(ctx, job.ID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", errors.New("no execution record for this job; logs are only available after a run or while running on a worker")
		}
		return "", err
	}
	if exec.MachineID == "" {
		return "", errors.New("execution record has no machine id")
	}
	return exec.MachineID, nil
}

type QueueHandler struct {
	engine *queue.Engine
	stats  *queue.StatsAggregator
}

func NewQueueHandler(engine *queue.Engine, stats *queue.StatsAggregator) *QueueHandler {
	return &QueueHandler{engine: engine, stats: stats}
}

func (h *QueueHandler) ListQueues(c *gin.Context) {
	ctx := c.Request.Context()
	names, err := h.engine.ListQueues(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type queueInfo struct {
		Name  string `json:"name"`
		Depth int64  `json:"depth"`
	}

	queues := make([]queueInfo, 0, len(names))
	for _, name := range names {
		depth, _ := h.engine.QueueDepth(ctx, name)
		queues = append(queues, queueInfo{Name: name, Depth: depth})
	}

	c.JSON(http.StatusOK, gin.H{"queues": queues})
}

func (h *QueueHandler) ListQueueJobs(c *gin.Context) {
	name := c.Param("name")
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)

	ctx := c.Request.Context()
	jobIDs, err := h.engine.ListQueueJobs(ctx, name, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	jobList := make([]*queue.Job, 0, len(jobIDs))
	for _, id := range jobIDs {
		job, err := h.engine.GetJob(ctx, id)
		if err == nil {
			jobList = append(jobList, job)
		}
	}

	c.JSON(http.StatusOK, gin.H{"jobs": jobList, "count": len(jobList)})
}

func (h *QueueHandler) PurgeQueue(c *gin.Context) {
	name := c.Param("name")
	count, err := h.engine.PurgeQueue(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Queue purged", "deleted": count})
}

func (h *QueueHandler) GetStats(c *gin.Context) {
	name := c.Param("name")
	minutes, _ := strconv.Atoi(c.DefaultQuery("minutes", "60"))
	if minutes <= 0 || minutes > 120 {
		minutes = 60
	}

	stats, err := h.stats.GetTimeSeries(c.Request.Context(), name, minutes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"queue": name, "stats": stats})
}

func (h *QueueHandler) GetAllStats(c *gin.Context) {
	ctx := c.Request.Context()
	names, err := h.engine.ListQueues(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	minutes, _ := strconv.Atoi(c.DefaultQuery("minutes", "60"))
	if minutes <= 0 || minutes > 120 {
		minutes = 60
	}

	result := make(map[string][]queue.QueueStats)
	for _, name := range names {
		stats, err := h.stats.GetTimeSeries(ctx, name, minutes)
		if err == nil {
			result[name] = stats
		}
	}

	c.JSON(http.StatusOK, gin.H{"stats": result})
}

type DLQHandler struct {
	engine  *queue.Engine
	grpcSrv *grpc.Server
}

func NewDLQHandler(engine *queue.Engine, grpcSrv *grpc.Server) *DLQHandler {
	return &DLQHandler{engine: engine, grpcSrv: grpcSrv}
}

func (h *DLQHandler) ListDLQ(c *gin.Context) {
	queueName := c.Param("queue")
	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)

	ctx := c.Request.Context()
	jobIDs, err := h.engine.DLQList(ctx, queueName, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	jobList := make([]*queue.Job, 0, len(jobIDs))
	for _, id := range jobIDs {
		job, err := h.engine.GetJob(ctx, id)
		if err == nil {
			jobList = append(jobList, job)
		}
	}

	c.JSON(http.StatusOK, gin.H{"jobs": jobList, "count": len(jobList)})
}

func (h *DLQHandler) RetryAll(c *gin.Context) {
	queueName := c.Param("queue")
	ctx := c.Request.Context()
	count, err := h.engine.DLQRetryAll(ctx, queueName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.grpcSrv != nil && count > 0 {
		h.grpcSrv.DispatchQueue(ctx, queueName)
	}
	c.JSON(http.StatusOK, gin.H{"message": "DLQ retried", "count": count})
}

type WorkersInfoHandler struct {
	connMgr *grpc.ConnectionManager
}

func NewWorkersInfoHandler(connMgr *grpc.ConnectionManager) *WorkersInfoHandler {
	return &WorkersInfoHandler{connMgr: connMgr}
}

func (h *WorkersInfoHandler) ListWorkers(c *gin.Context) {
	workers := h.connMgr.ActiveWorkers()
	c.JSON(http.StatusOK, gin.H{
		"workers": workers,
		"count":   len(workers),
	})
}
