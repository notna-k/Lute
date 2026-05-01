package worker

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ListConnectedWorkers handles GET /api/v1/workers/connected (authenticated).
func (h *WorkerHandler) ListConnectedWorkers(c *gin.Context) {
	if h.connMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "connection manager unavailable"})
		return
	}
	w := h.connMgr.ActiveWorkers()
	c.JSON(http.StatusOK, gin.H{
		"workers": w,
		"count":   len(w),
	})
}
