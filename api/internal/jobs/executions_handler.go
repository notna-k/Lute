package jobs

import (
	"net/http"
	"strconv"
	"strings"

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

	// queue and type repeat (?queue=a&queue=b): the panel's facets are
	// multi-select, and repeating the key keeps a value containing a comma
	// intact.
	filter := repos.JobExecutionListFilter{
		Queues: values(c, "queue"),
		Types:  values(c, "type"),
		Status: c.Query("status"),
		Search: c.Query("q"),
	}

	offset, _ := strconv.ParseInt(c.DefaultQuery("offset", "0"), 10, 64)
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)

	order, ok := repos.JobExecutionOrders[c.DefaultQuery("sort", "finished_at_desc")]
	if !ok {
		order = repos.DefaultJobExecutionOrder
	}

	execs, total, err := h.repo.List(c.Request.Context(), filter, offset, limit, order)
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

// values reads a repeated query parameter, dropping blanks so a stale
// "?queue=" from a cleared filter does not exclude every row.
func values(c *gin.Context, key string) []string {
	var out []string
	for _, v := range c.QueryArray(key) {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
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
