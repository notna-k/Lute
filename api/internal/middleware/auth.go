package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/auth"
	"github.com/lute/api/internal/httpx"
)

// JWTAuthMiddleware authenticates a Bearer access JWT and sets user_id and email.
func JWTAuthMiddleware(tokens *auth.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := extractBearer(c.GetHeader("Authorization"))
		if raw == "" {
			httpx.Error(c, http.StatusUnauthorized, "missing Bearer token")
			return
		}
		claims, err := tokens.ParseAccess(raw)
		if err != nil {
			httpx.Error(c, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}
