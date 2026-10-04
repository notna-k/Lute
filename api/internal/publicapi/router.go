package publicapi

import (
	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/middleware"
	"github.com/lute/api/internal/worker"
)

// Handlers are the public API's own handlers; worker routes come from the worker package.
type Handlers struct {
	Runs *RunsHandler
	Jobs *JobsHandler
	Meta *MetaHandler
}

// SetupPublicRoutes mounts /api/public/v1. Every route but /version needs an API key, and
// worker routes need an account key, since workers belong to a user.
func SetupPublicRoutes(r *gin.RouterGroup, keyRepo *repos.APIKeyRepository, h Handlers, wh *worker.WorkerHandler) {
	r.GET("/version", h.Meta.Version)

	authed := r.Group("")
	authed.Use(middleware.APIKeyAuthMiddleware(keyRepo))

	authed.GET("/whoami", h.Meta.WhoAmI)

	jobsGroup := authed.Group("/jobs")
	{
		jobsGroup.GET("", h.Jobs.List)
		jobsGroup.GET("/:slug", h.Jobs.Get)
		jobsGroup.POST("/:slug/runs", h.Jobs.StartRun)
	}

	runsGroup := authed.Group("/runs")
	{
		runsGroup.POST("", h.Runs.Create)
		runsGroup.GET("", h.Runs.List)
		runsGroup.GET("/:id", h.Runs.Get)
		runsGroup.POST("/:id/retry", h.Runs.Retry)
		runsGroup.DELETE("/:id", h.Runs.Cancel)
		runsGroup.GET("/:id/logs", h.Runs.Logs)
	}

	worker.MountAPIKey(authed.Group("", middleware.RequireAccountKey()), wh)
}

// SetupAPIKeyRoutes mounts key management on a JWT-authenticated group.
func SetupAPIKeyRoutes(r *gin.RouterGroup, keys *APIKeysHandler) {
	g := r.Group("/api-keys")
	{
		g.POST("", keys.Create)
		g.GET("", keys.List)
		g.DELETE("/:id", keys.Revoke)
	}
}
