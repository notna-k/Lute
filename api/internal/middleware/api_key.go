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
	"github.com/lute/api/internal/httpx"
)

// APIKeyAuthMiddleware authenticates "Bearer lute_sk_..." and records the key with
// httpx.SetKeyCaller. last_used_at is updated in the background.
func APIKeyAuthMiddleware(keyRepo *repos.APIKeyRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearer(c.GetHeader("Authorization"))
		if token == "" {
			httpx.Error(c, http.StatusUnauthorized, "missing Bearer token")
			return
		}
		prefix := apikey.PrefixOf(token)
		if prefix == "" {
			httpx.Error(c, http.StatusUnauthorized, "invalid API key format")
			return
		}

		ctx := c.Request.Context()
		k, err := keyRepo.GetByPrefix(ctx, prefix)
		if err != nil {
			if errors.Is(err, repos.ErrNotFound) {
				httpx.Error(c, http.StatusUnauthorized, "invalid API key")
			} else {
				httpx.Internal(c, err)
			}
			return
		}

		computed := apikey.Hash(token)
		if subtle.ConstantTimeCompare([]byte(computed), []byte(k.Hash)) != 1 {
			httpx.Error(c, http.StatusUnauthorized, "invalid API key")
			return
		}

		httpx.SetKeyCaller(c, httpx.KeyCaller{KeyID: k.ID, Service: k.IsService(), UserID: k.UserID})

		keyID := k.ID
		go func() {
			bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = keyRepo.TouchUsed(bg, keyID)
		}()

		c.Next()
	}
}

// RequireAccountKey refuses service keys, for routes that act on a user's own resources.
func RequireAccountKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		k, ok := httpx.Key(c)
		if !ok {
			return
		}
		if k.Service {
			httpx.Error(c, http.StatusForbidden, "this route needs an account key; service keys cannot use it")
			return
		}
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
