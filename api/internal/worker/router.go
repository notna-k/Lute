package worker

import (
	"github.com/gin-gonic/gin"
)

// SetupRoutes registers JWT-authenticated /api/v1/workers routes for the UI.
// Worker bootstrap (install.sh, download, register, ...) lives on the public API
// only; it is mounted from publicapi.SetupPublicRoutes so unauthenticated
// machines have a single canonical entry point.
func SetupRoutes(r *gin.RouterGroup, h *WorkerHandler, authedMW gin.HandlerFunc) {
	MountWorkerJWTAPI(r, h, authedMW)
}
