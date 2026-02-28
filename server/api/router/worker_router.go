package router

import (
	"github.com/lute/api/handlers"
	"github.com/lute/api/middleware"
	"github.com/lute/api/repository"

	"github.com/gin-gonic/gin"
)

// SetupWorkerRoutes sets up worker binary distribution and registration routes
func SetupWorkerRoutes(r *gin.RouterGroup, workerHandler *handlers.WorkerHandler, userRepo *repository.UserRepository) {
	worker := r.Group("/worker")
	{
		// Public endpoints — workers on VMs need these without auth
		worker.GET("/download/:os/:arch", workerHandler.DownloadBinary)
		worker.GET("/download", workerHandler.DownloadAutoDetect)
		worker.GET("/version", workerHandler.GetVersion)
		worker.GET("/install.sh", workerHandler.InstallScript)
		worker.POST("/register", workerHandler.RegisterFromWorker)

		// Protected endpoints — only authenticated users
		protected := worker.Group("")
		protected.Use(middleware.AuthMiddleware(userRepo))
		{
			protected.GET("/binaries", workerHandler.ListBinaries)
			protected.POST("/refresh", workerHandler.RefreshBinaries)
			protected.POST("/claim-code", workerHandler.CreateClaimCode)

			// Worker management (UI-facing)
			protected.POST("/command/:machineId", workerHandler.SendCommand)
			protected.GET("/commands/:machineId", workerHandler.ListCommands)
			protected.GET("/command/:commandId", workerHandler.GetCommandResult)
			protected.GET("/status/:machineId", workerHandler.GetWorkerStatus)
		}
	}
}
