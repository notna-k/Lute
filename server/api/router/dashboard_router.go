package router

import (
	"github.com/lute/api/handlers"
	"github.com/lute/api/middleware"
	"github.com/lute/api/repository"

	"github.com/gin-gonic/gin"
)

// SetupDashboardRoutes sets up dashboard routes (stats, uptime). All require authentication.
func SetupDashboardRoutes(r *gin.RouterGroup, dashboardHandler *handlers.DashboardHandler, userRepo *repository.UserRepository) {
	dashboard := r.Group("/dashboard")
	dashboard.Use(middleware.AuthMiddleware(userRepo))
	{
		dashboard.GET("/config", dashboardHandler.GetConfig)
		dashboard.GET("/stats", dashboardHandler.GetStats)
		dashboard.GET("/uptime", dashboardHandler.GetUptime)
	}
}
