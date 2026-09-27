package worker

import "github.com/gin-gonic/gin"

// MountJWT registers worker management for the panel, behind authedMW.
func MountJWT(parent *gin.RouterGroup, h *WorkerHandler, authedMW gin.HandlerFunc) {
	mountManagement(parent.Group("/workers", authedMW), h)
}

// MountAPIKey registers worker management under a group that already enforces API-key auth.
func MountAPIKey(parent *gin.RouterGroup, h *WorkerHandler) {
	mountManagement(parent.Group("/workers"), h)
}

func mountManagement(g *gin.RouterGroup, h *WorkerHandler) {
	g.GET("/connected", h.ListConnectedWorkers)
	g.GET("/install", h.InstallInfo)

	g.GET("/tokens", h.ListTokens)
	g.POST("/tokens", h.CreateToken)
	g.DELETE("/tokens/:tokenId", h.RevokeToken)

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
