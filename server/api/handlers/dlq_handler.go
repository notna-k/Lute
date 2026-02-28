package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/queue"
)

type DLQHandler struct {
	engine *queue.Engine
}

func NewDLQHandler(engine *queue.Engine) *DLQHandler {
	return &DLQHandler{engine: engine}
}

// ListDLQ returns jobs in the dead letter queue.
// GET /api/v1/dlq/:queue
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

	jobs := make([]*queue.Job, 0, len(jobIDs))
	for _, id := range jobIDs {
		job, err := h.engine.GetJob(ctx, id)
		if err == nil {
			jobs = append(jobs, job)
		}
	}

	c.JSON(http.StatusOK, gin.H{"jobs": jobs, "count": len(jobs)})
}

// RetryAll moves all DLQ jobs back to their queue.
// POST /api/v1/dlq/:queue/retry-all
func (h *DLQHandler) RetryAll(c *gin.Context) {
	queueName := c.Param("queue")
	count, err := h.engine.DLQRetryAll(c.Request.Context(), queueName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "DLQ retried", "count": count})
}
