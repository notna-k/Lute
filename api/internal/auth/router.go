package auth

import "github.com/gin-gonic/gin"

func SetupRoutes(parent *gin.RouterGroup, h *Handler, authedMW gin.HandlerFunc) {
	g := parent.Group("/auth")
	g.POST("/login", h.Login)
	g.POST("/refresh", h.Refresh)
	g.POST("/logout", h.Logout)
	g.GET("/me", authedMW, h.Me)
}
