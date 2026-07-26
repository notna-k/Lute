package jobdefs

import "github.com/gin-gonic/gin"

// SetupRoutes mounts job-definition routes under an already-authenticated group.
// Path is /job-definitions (not /jobs) to avoid colliding with the legacy
// queue-job routes in the `jobs` package.
func SetupRoutes(authed *gin.RouterGroup, h *Handler) {
	jobs := authed.Group("/job-definitions")
	{
		jobs.GET("", h.List)
		jobs.POST("", h.Create)
		// Registered before /:slug/... so "adhoc" is never read as a slug.
		jobs.POST("/adhoc/trigger", h.TriggerAdhoc)
		jobs.GET("/:slug", h.Get)
		jobs.PUT("/:slug", h.Update)
		jobs.GET("/:slug/builds", h.Builds)
		jobs.POST("/:slug/trigger", h.Trigger)
	}
}
