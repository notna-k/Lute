package worker

import (
	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/middleware"
)

// MountWorkerBootstrap registers unauthenticated /workers/bootstrap/* under parent
// (e.g. parent is /api/v1 or /api/public/v1).
func MountWorkerBootstrap(parent *gin.RouterGroup, h *WorkerHandler) {
	w := parent.Group("/workers")
	boot := w.Group("/bootstrap")
	{
		boot.GET("/install.sh", h.InstallScript)
		boot.GET("/version", h.GetVersion)
		boot.GET("/download/:os/:arch", h.DownloadBinary)
		boot.GET("/download", h.DownloadAutoDetect)
		boot.POST("/register", h.RegisterFromWorker)
	}
}

// MountWorkerJWTAPI registers Firebase-authenticated worker routes under parent.
func MountWorkerJWTAPI(parent *gin.RouterGroup, h *WorkerHandler, userRepo *repos.UserRepository) {
	w := parent.Group("/workers")
	authd := w.Group("")
	authd.Use(middleware.AuthMiddleware(userRepo))
	mountWorkerAuthenticated(authd, h)
}

// MountWorkerAPIKeyAPI registers worker routes under parent, which must already apply API key auth.
func MountWorkerAPIKeyAPI(parent *gin.RouterGroup, h *WorkerHandler) {
	w := parent.Group("/workers")
	mountWorkerAuthenticated(w, h)
}

func mountWorkerAuthenticated(g *gin.RouterGroup, h *WorkerHandler) {
	g.POST("/claim-code", h.CreateClaimCode)
	g.GET("/connected", h.ListConnectedWorkers)
	g.GET("/bootstrap/binaries", h.ListBinaries)
	g.POST("/bootstrap/binaries/refresh", h.RefreshBinaries)

	g.POST("", h.CreateWorker)
	g.GET("", h.ListUserWorkers)
	g.GET("/command-results/:commandId", h.GetCommandResult)
	g.POST("/:id/commands", h.SendCommand)
	g.GET("/:id/commands", h.ListCommands)
	g.GET("/:id/status", h.GetWorkerLiveStatus)
	g.GET("/:id", h.GetWorker)
	g.PUT("/:id", h.UpdateWorker)
	g.POST("/:id/re-enable", h.ReEnableWorker)
	g.DELETE("/:id", h.DeleteWorker)
}
