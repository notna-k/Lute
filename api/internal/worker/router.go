package worker

import (
	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/middleware"
)

// SetupRoutes registers all /api/v1/workers routes (bootstrap, CRUD, commands, connected).
func SetupRoutes(r *gin.RouterGroup, h *WorkerHandler, userRepo *repos.UserRepository) {
	g := r.Group("/workers")
	{
		g.GET("/public", h.ListPublicWorkers)

		boot := g.Group("/bootstrap")
		{
			boot.GET("/install.sh", h.InstallScript)
			boot.GET("/version", h.GetVersion)
			boot.GET("/download/:os/:arch", h.DownloadBinary)
			boot.GET("/download", h.DownloadAutoDetect)
			boot.POST("/register", h.RegisterFromWorker)
		}

		authd := g.Group("")
		authd.Use(middleware.AuthMiddleware(userRepo))
		{
			authd.POST("/claim-code", h.CreateClaimCode)
			authd.GET("/connected", h.ListConnectedWorkers)
			authd.GET("/bootstrap/binaries", h.ListBinaries)
			authd.POST("/bootstrap/binaries/refresh", h.RefreshBinaries)

			authd.POST("", h.CreateWorker)
			authd.GET("", h.ListUserWorkers)
			authd.GET("/command-results/:commandId", h.GetCommandResult)
			authd.POST("/:id/commands", h.SendCommand)
			authd.GET("/:id/commands", h.ListCommands)
			authd.GET("/:id/status", h.GetWorkerLiveStatus)
			authd.GET("/:id", h.GetWorker)
			authd.PUT("/:id", h.UpdateWorker)
			authd.POST("/:id/re-enable", h.ReEnableWorker)
			authd.DELETE("/:id", h.DeleteWorker)
		}
	}
}
