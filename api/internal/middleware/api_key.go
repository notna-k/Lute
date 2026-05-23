package middleware

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/apikey"
	"github.com/lute/api/internal/db/repos"
)

// APIKeyAuthMiddleware validates an "Authorization: Bearer lute_sk_..." token
// and stores the owning user id on the context. The last_used_at timestamp
// is updated best-effort without blocking the request.
func APIKeyAuthMiddleware(keyRepo *repos.APIKeyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearer(c.GetHeader("Authorization"))
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing Bearer token"})
			c.Abort()
			return
		}
		prefix := apikey.PrefixOf(token)
		if prefix == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid API key format"})
			c.Abort()
			return
		}

		ctx := c.Request.Context()
		k, err := keyRepo.GetByPrefix(ctx, prefix)
		if err != nil {
			if errors.Is(err, repos.ErrNotFound) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "auth lookup failed"})
			}
			c.Abort()
			return
		}

		computed := apikey.Hash(token)
		if subtle.ConstantTimeCompare([]byte(computed), []byte(k.Hash)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			c.Abort()
			return
		}

		c.Set("user_id", k.UserID.Hex())
		c.Set("api_key_id", k.ID.Hex())

		keyID := k.ID
		go func() {
			bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = keyRepo.TouchUsed(bg, keyID)
		}()

		c.Next()
	}
}

func extractBearer(h string) string {
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
