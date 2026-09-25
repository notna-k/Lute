package settings

import "github.com/gin-gonic/gin"

// SetupRoutes lets every signed-in user read and write settings; there are no roles yet.
func SetupRoutes(authed *gin.RouterGroup, h *Handler) {
	s := authed.Group("/settings")
	{
		s.GET("", h.Get)
		s.PUT("", h.Update)
	}
}
