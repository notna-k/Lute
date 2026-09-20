package health

import (
	"github.com/gin-gonic/gin"
)

// SetupRoutes registers health and readiness routes on the given group.
func SetupRoutes(r *gin.RouterGroup, healthHandler *HealthHandler) {
	r.GET("/health", healthHandler.HealthCheck)
	r.GET("/ready", healthHandler.Readiness)
}
