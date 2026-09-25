package publicapi

import (
	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/middleware"
	"github.com/lute/api/internal/worker"
)

// SetupPublicRoutes mounts /api/public/v1: worker bootstrap is open, everything else needs an API key.
func SetupPublicRoutes(r *gin.RouterGroup, keyRepo *repos.APIKeyRepository, runs *RunsHandler, wh *worker.WorkerHandler) {
	worker.MountBootstrap(r, wh)

	authed := r.Group("")
	authed.Use(middleware.APIKeyAuthMiddleware(keyRepo))

	runsGroup := authed.Group("/runs")
	{
		runsGroup.POST("", runs.Create)
		runsGroup.GET("", runs.List)
		runsGroup.GET("/:id", runs.Get)
		runsGroup.POST("/:id/retry", runs.Retry)
		runsGroup.DELETE("/:id", runs.Cancel)
		runsGroup.GET("/:id/logs", runs.Logs)
	}

	worker.MountAPIKey(authed, wh)
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
