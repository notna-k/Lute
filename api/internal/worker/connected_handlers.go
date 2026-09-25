package worker

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/httpx"
)

func (h *WorkerHandler) ListConnectedWorkers(c *gin.Context) {
	if h.connectionMgr == nil {
		httpx.Error(c, http.StatusServiceUnavailable, "connection manager unavailable")
		return
	}
	w := h.connectionMgr.ActiveWorkers()
	c.JSON(http.StatusOK, gin.H{
		"workers": w,
		"count":   len(w),
	})
}
