package jobs

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
)

type ExecutionsHandler struct {
	repo *repos.JobExecutionRepository
}

func NewExecutionsHandler(repo *repos.JobExecutionRepository) *ExecutionsHandler {
	return &ExecutionsHandler{repo: repo}
}

// ListExecutions returns paginated rows from job_executions with optional filters.
func (h *ExecutionsHandler) ListExecutions(c *gin.Context) {
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "executions store unavailable"})
		return
	}

	filter := repos.JobExecutionListFilter{
		Queue:  c.Query("queue"),
		Type:   c.Query("type"),
		Status: c.Query("status"),
	}

	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)

	sort := c.DefaultQuery("sort", "finished_at_desc")
	sortDesc := sort != "finished_at_asc"

	execs, total, err := h.repo.List(c.Request.Context(), filter, offset, limit, sortDesc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"executions": execs,
		"total":      total,
		"offset":     offset,
		"limit":      limit,
	})
}

// ExecutionFilterOptions returns distinct queue and type values.
func (h *ExecutionsHandler) ExecutionFilterOptions(c *gin.Context) {
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "executions store unavailable"})
		return
	}
	queues, types, err := h.repo.DistinctQueuesAndTypes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"queues": queues,
		"types":  types,
	})
}
