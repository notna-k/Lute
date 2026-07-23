package jobdefs

import "github.com/gin-gonic/gin"

// SetupRoutes mounts job-definition routes under an already-authenticated group.
// Path is /job-definitions (not /jobs) to avoid colliding with the legacy
// queue-job routes in the `jobs` package.
func SetupRoutes(authed *gin.RouterGroup, h *Handler) {
	jobs := authed.Group("/job-definitions")
	{
		jobs.GET("", h.List)
		jobs.GET("/:slug", h.Get)
		jobs.GET("/:slug/builds", h.Builds)
		jobs.POST("/:slug/trigger", h.Trigger)
	}
}
