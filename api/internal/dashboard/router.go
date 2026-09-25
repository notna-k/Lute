package dashboard

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.RouterGroup, dashboardHandler *DashboardHandler, authedMW gin.HandlerFunc) {
	dashboard := r.Group("/dashboard")
	dashboard.Use(authedMW)
	{
		dashboard.GET("/config", dashboardHandler.GetConfig)
		dashboard.GET("/stats", dashboardHandler.GetStats)
		dashboard.GET("/uptime", dashboardHandler.GetUptime)
	}
}
