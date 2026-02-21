package router

import (
	"github.com/lute/api/handlers"
	"github.com/lute/api/middleware"
	"github.com/lute/api/repository"

	"github.com/gin-gonic/gin"
)

// SetupAgentRoutes sets up agent binary distribution and registration routes
func SetupAgentRoutes(r *gin.RouterGroup, agentHandler *handlers.AgentHandler, userRepo *repository.UserRepository) {
	agent := r.Group("/agent")
	{
		// Public endpoints — agents on VMs need these without auth
		agent.GET("/download/:os/:arch", agentHandler.DownloadBinary)
		agent.GET("/download", agentHandler.DownloadAutoDetect)
		agent.GET("/version", agentHandler.GetVersion)
		agent.GET("/install.sh", agentHandler.InstallScript)
		agent.POST("/register", agentHandler.RegisterFromAgent)

		// Protected endpoints — only authenticated users
		protected := agent.Group("")
		protected.Use(middleware.AuthMiddleware(userRepo))
		{
			protected.GET("/binaries", agentHandler.ListBinaries)
			protected.POST("/refresh", agentHandler.RefreshBinaries)
			protected.POST("/claim-code", agentHandler.CreateClaimCode)

			// Agent management (UI-facing)
			protected.POST("/command/:machineId", agentHandler.SendCommand)
			protected.GET("/commands/:machineId", agentHandler.ListCommands)
			protected.GET("/command/:commandId", agentHandler.GetCommandResult)
			protected.GET("/status/:machineId", agentHandler.GetAgentStatus)
		}
	}
}
