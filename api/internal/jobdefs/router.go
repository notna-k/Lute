package jobdefs

import "github.com/gin-gonic/gin"

// SetupRoutes mounts /job-definitions, distinct from the queue-job routes under /jobs.
func SetupRoutes(authed *gin.RouterGroup, h *Handler) {
	jobs := authed.Group("/job-definitions")
	{
		jobs.GET("", h.List)
		jobs.POST("", h.Create)
		jobs.POST("/sync", h.Sync)
		jobs.GET("/export", h.Export)
		jobs.GET("/export.zip", h.ExportZip)
		jobs.GET("/:slug", h.Get)
		jobs.PUT("/:slug", h.Update)
		jobs.GET("/:slug/yaml", h.ExportOne)
		jobs.POST("/:slug/revert", h.Revert)
		jobs.GET("/:slug/builds", h.Builds)
		jobs.POST("/:slug/trigger", h.Trigger)
	}
}
