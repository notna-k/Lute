package publicapi

import (
	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/middleware"
	"github.com/lute/api/internal/worker"
)

// SetupPublicRoutes registers the public API under the given router group.
// Callers should pass a group mounted at /api/public/v1.
// Bootstrap worker routes are unauthenticated; runs and worker management require an API key.
func SetupPublicRoutes(r *gin.RouterGroup, keyRepo *repos.APIKeyRepository, runs *RunsHandler, wh *worker.WorkerHandler) {
	worker.MountWorkerBootstrap(r, wh)

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

	worker.MountWorkerAPIKeyAPI(authed, wh)
}

// SetupAPIKeyRoutes registers key management endpoints on an already
// Firebase-authenticated group (e.g. /api/v1/api-keys).
func SetupAPIKeyRoutes(r *gin.RouterGroup, keys *APIKeysHandler) {
	g := r.Group("/api-keys")
	{
		g.POST("", keys.Create)
		g.GET("", keys.List)
		g.DELETE("/:id", keys.Revoke)
	}
}
