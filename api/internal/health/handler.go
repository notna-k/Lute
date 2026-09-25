package health

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/httpx"
)

type HealthHandler struct {
	db *connection.Database
}

func NewHealthHandler(db *connection.Database) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) HealthCheck(c *gin.Context) {
	ctx := c.Request.Context()

	if err := h.db.HealthCheck(ctx); err != nil {
		httpx.Error(c, http.StatusServiceUnavailable, "database unreachable: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "lute-api",
	})
}

func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx := context.Background()

	if err := h.db.HealthCheck(ctx); err != nil {
		httpx.Error(c, http.StatusServiceUnavailable, "database unreachable: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ready": true,
	})
}
