package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	luteGrpc "github.com/lute/api/grpc"
)

type WorkersInfoHandler struct {
	connMgr *luteGrpc.ConnectionManager
}

func NewWorkersInfoHandler(connMgr *luteGrpc.ConnectionManager) *WorkersInfoHandler {
	return &WorkersInfoHandler{connMgr: connMgr}
}

// ListWorkers returns connected workers and their utilization.
// GET /api/v1/workers
func (h *WorkersInfoHandler) ListWorkers(c *gin.Context) {
	workers := h.connMgr.ActiveWorkers()
	c.JSON(http.StatusOK, gin.H{
		"workers": workers,
		"count":   len(workers),
	})
}
