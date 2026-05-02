package health

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/connection"
)

type HealthHandler struct {
	db *connection.SQLite
}

func NewHealthHandler(db *connection.SQLite) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) HealthCheck(c *gin.Context) {
	ctx := c.Request.Context()

	if err := h.db.HealthCheck(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
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
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"ready": false,
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ready": true,
	})
}
