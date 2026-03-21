package worker

import (
	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/middleware"
)

// SetupRoutes sets up worker binary distribution and registration routes.
func SetupRoutes(r *gin.RouterGroup, workerHandler *WorkerHandler, userRepo *repos.UserRepository) {
	worker := r.Group("/worker")
	{
		worker.GET("/download/:os/:arch", workerHandler.DownloadBinary)
		worker.GET("/download", workerHandler.DownloadAutoDetect)
		worker.GET("/version", workerHandler.GetVersion)
		worker.GET("/install.sh", workerHandler.InstallScript)
		worker.POST("/register", workerHandler.RegisterFromWorker)

		protected := worker.Group("")
		protected.Use(middleware.AuthMiddleware(userRepo))
		{
			protected.GET("/binaries", workerHandler.ListBinaries)
			protected.POST("/refresh", workerHandler.RefreshBinaries)
			protected.POST("/claim-code", workerHandler.CreateClaimCode)

			protected.POST("/command/:machineId", workerHandler.SendCommand)
			protected.GET("/commands/:machineId", workerHandler.ListCommands)
			protected.GET("/command/:commandId", workerHandler.GetCommandResult)
			protected.GET("/status/:machineId", workerHandler.GetWorkerStatus)
		}
	}
}
