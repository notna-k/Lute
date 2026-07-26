package settings

import "github.com/gin-gonic/gin"

// SetupRoutes mounts settings routes under an already-authenticated group.
//
// Note: every authenticated user may read *and* write these. The users table
// has no role column today, so there is no admin/viewer distinction to enforce
// — worth revisiting when roles land.
func SetupRoutes(authed *gin.RouterGroup, h *Handler) {
	s := authed.Group("/settings")
	{
		s.GET("", h.Get)
		s.PUT("", h.Update)
	}
}
