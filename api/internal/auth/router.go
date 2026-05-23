package auth

import "github.com/gin-gonic/gin"

// SetupRoutes mounts /auth endpoints. parent should be /api/v1.
// authedMW is the JWT middleware used to gate /auth/me.
func SetupRoutes(parent *gin.RouterGroup, h *Handler, authedMW gin.HandlerFunc) {
	g := parent.Group("/auth")
	g.POST("/login", h.Login)
	g.POST("/refresh", h.Refresh)
	g.POST("/logout", h.Logout)
	g.GET("/me", authedMW, h.Me)
}
