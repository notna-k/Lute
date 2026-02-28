package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/queue"
)

type QueueHandler struct {
	engine *queue.Engine
	stats  *queue.StatsAggregator
}

func NewQueueHandler(engine *queue.Engine, stats *queue.StatsAggregator) *QueueHandler {
	return &QueueHandler{engine: engine, stats: stats}
}

// ListQueues returns all queues with depth and stats.
// GET /api/v1/queues
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

// ListQueueJobs returns paginated jobs from a queue.
// GET /api/v1/queues/:name/jobs
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

	jobs := make([]*queue.Job, 0, len(jobIDs))
	for _, id := range jobIDs {
		job, err := h.engine.GetJob(ctx, id)
		if err == nil {
			jobs = append(jobs, job)
		}
	}

	c.JSON(http.StatusOK, gin.H{"jobs": jobs, "count": len(jobs)})
}

// PurgeQueue removes all pending jobs from a queue.
// POST /api/v1/queues/:name/purge
func (h *QueueHandler) PurgeQueue(c *gin.Context) {
	name := c.Param("name")
	count, err := h.engine.PurgeQueue(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Queue purged", "deleted": count})
}

// GetStats returns time series stats for a queue.
// GET /api/v1/stats/queues/:name
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

// GetAllStats returns time series stats for all queues.
// GET /api/v1/stats/queues
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
