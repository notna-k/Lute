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

func (h *ExecutionsHandler) ListExecutions(c *gin.Context) {
	if h.repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "executions store unavailable"})
		return
	}

	// Facets repeat the key (?queue=a&queue=b) so a value containing a comma survives.
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

// values drops blanks, so a stale "?queue=" from a cleared filter does not exclude every row.
func values(c *gin.Context, key string) []string {
	var out []string
	for _, v := range c.QueryArray(key) {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

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
