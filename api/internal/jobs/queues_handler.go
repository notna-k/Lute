package jobs

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/httpx"
	"github.com/lute/api/internal/queue"
)

type QueueHandler struct {
	engine *queue.Engine
	stats  *queue.Stats
}

func NewQueueHandler(engine *queue.Engine, stats *queue.Stats) *QueueHandler {
	return &QueueHandler{engine: engine, stats: stats}
}

func (h *QueueHandler) ListQueues(c *gin.Context) {
	ctx := c.Request.Context()
	names, err := h.engine.ListQueues(ctx)
	if err != nil {
		httpx.Internal(c, err)
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
		httpx.Internal(c, err)
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
		httpx.Internal(c, err)
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
		httpx.Internal(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"queue": name, "stats": stats})
}

func (h *QueueHandler) GetAllStats(c *gin.Context) {
	ctx := c.Request.Context()
	names, err := h.engine.ListQueues(ctx)
	if err != nil {
		httpx.Internal(c, err)
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
		httpx.Internal(c, err)
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
		httpx.Internal(c, err)
		return
	}
	if h.grpcSrv != nil && count > 0 {
		h.grpcSrv.DispatchQueue(ctx, queueName)
	}
	c.JSON(http.StatusOK, gin.H{"message": "DLQ retried", "count": count})
}
