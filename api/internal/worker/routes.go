package worker

import "github.com/gin-gonic/gin"

// MountBootstrap registers the unauthenticated /workers/bootstrap routes a new host uses
// to install the agent and register. They live on the public API only.
func MountBootstrap(parent *gin.RouterGroup, h *WorkerHandler) {
	boot := parent.Group("/workers/bootstrap")
	boot.GET("/install.sh", h.InstallScript)
	boot.GET("/version", h.GetVersion)
	boot.GET("/download/:os/:arch", h.DownloadBinary)
	boot.GET("/download", h.DownloadAutoDetect)
	boot.POST("/register", h.RegisterFromWorker)
}

// MountJWT registers worker management for the panel, behind authedMW.
func MountJWT(parent *gin.RouterGroup, h *WorkerHandler, authedMW gin.HandlerFunc) {
	mountManagement(parent.Group("/workers", authedMW), h)
}

// MountAPIKey registers worker management under a group that already enforces API-key auth.
func MountAPIKey(parent *gin.RouterGroup, h *WorkerHandler) {
	mountManagement(parent.Group("/workers"), h)
}

func mountManagement(g *gin.RouterGroup, h *WorkerHandler) {
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
	g.GET("/:id/labels", h.GetLabels)
	g.PATCH("/:id/labels", h.PatchLabels)
	g.POST("/:id/re-enable", h.ReEnableWorker)
	g.DELETE("/:id", h.DeleteWorker)
}
