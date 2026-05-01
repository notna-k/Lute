package worker

import (
	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
)

// SetupRoutes registers Firebase-authenticated /api/v1/workers routes for the UI.
// Worker bootstrap (install.sh, download, register, ...) lives on the public API
// only; it is mounted from publicapi.SetupPublicRoutes so unauthenticated
// machines have a single canonical entry point.
func SetupRoutes(r *gin.RouterGroup, h *WorkerHandler, userRepo *repos.UserRepository) {
	MountWorkerJWTAPI(r, h, userRepo)
}
