package publicapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/apikey"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/httpx"
)

// APIKeysHandler manages public-API keys from the panel: the caller's own account keys,
// and the instance's service keys, which every signed-in user may manage until roles
// exist. Create shows the plaintext once.
type APIKeysHandler struct {
	repo  *repos.APIKeyRepository
	users *repos.UserRepository
}

func NewAPIKeysHandler(repo *repos.APIKeyRepository, users *repos.UserRepository) *APIKeysHandler {
	return &APIKeysHandler{repo: repo, users: users}
}

type createKeyRequest struct {
	Name string `json:"name" binding:"required"`
	// Scope is "account" (the default) or "service".
	Scope string `json:"scope"`
}

type createKeyResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Scope     string `json:"scope"`
	Prefix    string `json:"prefix"`
	Token     string `json:"token"`
	CreatedAt string `json:"created_at"`
}

type keySummary struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Scope        string  `json:"scope"`
	Prefix       string  `json:"prefix"`
	CreatedBy    string  `json:"created_by,omitempty"`
	CreatorEmail string  `json:"created_by_email,omitempty"`
	CreatedAt    string  `json:"created_at"`
	LastUsedAt   *string `json:"last_used_at,omitempty"`
	Revoked      bool    `json:"revoked"`
}

const rfc3339 = "2006-01-02T15:04:05Z07:00"

// readScope accepts "account", "service" or nothing (account), else answers 400.
func readScope(c *gin.Context, scope string) (string, bool) {
	switch scope {
	case "", models.KeyScopeAccount:
		return models.KeyScopeAccount, true
	case models.KeyScopeService:
		return models.KeyScopeService, true
	}
	httpx.Invalid(c, "scope must be account or service", map[string]string{"scope": "must be account or service"})
	return "", false
}

func (h *APIKeysHandler) Create(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	var req createKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	scope, ok := readScope(c, req.Scope)
	if !ok {
		return
	}

	token, prefix, hash, err := apikey.Generate()
	if err != nil {
		httpx.Internal(c, err)
		return
	}

	k := &models.APIKey{
		Scope:     scope,
		CreatedBy: userID,
		Name:      req.Name,
		Prefix:    prefix,
		Hash:      hash,
	}
	if scope == models.KeyScopeAccount {
		k.UserID = userID
	}
	if err := h.repo.Create(c.Request.Context(), k); err != nil {
		httpx.Internal(c, err)
		return
	}

	c.JSON(http.StatusCreated, createKeyResponse{
		ID:        k.ID.Hex(),
		Name:      k.Name,
		Scope:     k.Scope,
		Prefix:    k.Prefix,
		Token:     token,
		CreatedAt: k.CreatedAt.UTC().Format(rfc3339),
	})
}

// List answers ?scope=account (the default) with the caller's own keys, and
// ?scope=service with every service key.
func (h *APIKeysHandler) List(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	scope, ok := readScope(c, c.Query("scope"))
	if !ok {
		return
	}
	ctx := c.Request.Context()
	var keys []*models.APIKey
	var err error
	if scope == models.KeyScopeService {
		keys, err = h.repo.ListService(ctx)
	} else {
		keys, err = h.repo.ListByUser(ctx, userID)
	}
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	emails, err := h.creatorEmails(ctx, keys)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	out := make([]keySummary, 0, len(keys))
	for _, k := range keys {
		s := keySummary{
			ID:           k.ID.Hex(),
			Name:         k.Name,
			Scope:        k.Scope,
			Prefix:       k.Prefix,
			CreatedBy:    k.CreatedBy.Hex(),
			CreatorEmail: emails[k.CreatedBy],
			CreatedAt:    k.CreatedAt.UTC().Format(rfc3339),
			Revoked:      k.RevokedAt != nil,
		}
		if k.LastUsedAt != nil {
			t := k.LastUsedAt.UTC().Format(rfc3339)
			s.LastUsedAt = &t
		}
		out = append(out, s)
	}
	c.JSON(http.StatusOK, gin.H{"api_keys": out})
}

func (h *APIKeysHandler) creatorEmails(ctx context.Context, keys []*models.APIKey) (map[id.ID]string, error) {
	ids := make([]id.ID, 0, len(keys))
	for _, k := range keys {
		if !k.CreatedBy.IsZero() {
			ids = append(ids, k.CreatedBy)
		}
	}
	return h.users.Emails(ctx, ids)
}

func (h *APIKeysHandler) Revoke(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	keyID, err := id.FromHex(c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.repo.Revoke(c.Request.Context(), keyID, userID); err != nil {
		if errors.Is(err, repos.ErrNotFound) {
			httpx.Error(c, http.StatusNotFound, "API key not found")
			return
		}
		httpx.Internal(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
