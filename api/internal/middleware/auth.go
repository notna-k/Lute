package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/auth"
)

// JWTAuthMiddleware authenticates a Bearer access JWT and sets user_id and email.
func JWTAuthMiddleware(tokens *auth.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := extractBearer(c.GetHeader("Authorization"))
		if raw == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing Bearer token"})
			c.Abort()
			return
		}
		claims, err := tokens.ParseAccess(raw)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}
