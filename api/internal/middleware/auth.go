package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/auth"
)

// JWTAuthMiddleware verifies an "Authorization: Bearer <access-jwt>" header,
// validates the signature and expiry, and stores user_id on the gin context.
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

// OptionalAuthMiddleware records the bearer token on the context when present
// (used by the WS upgrade path which authenticates later).
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h := c.GetHeader("Authorization"); h != "" {
			parts := strings.SplitN(h, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				c.Set("token", strings.TrimSpace(parts[1]))
			}
		}
		c.Next()
	}
}
