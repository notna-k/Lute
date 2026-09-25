package health

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.RouterGroup, healthHandler *HealthHandler) {
	r.GET("/health", healthHandler.HealthCheck)
	r.GET("/ready", healthHandler.Readiness)
}
