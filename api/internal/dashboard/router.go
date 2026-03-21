package dashboard

import (
	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/middleware"
)

// SetupRoutes sets up dashboard routes (stats, uptime). All require authentication.
func SetupRoutes(r *gin.RouterGroup, dashboardHandler *DashboardHandler, userRepo *repos.UserRepository) {
	dashboard := r.Group("/dashboard")
	dashboard.Use(middleware.AuthMiddleware(userRepo))
	{
		dashboard.GET("/config", dashboardHandler.GetConfig)
		dashboard.GET("/stats", dashboardHandler.GetStats)
		dashboard.GET("/uptime", dashboardHandler.GetUptime)
	}
}
