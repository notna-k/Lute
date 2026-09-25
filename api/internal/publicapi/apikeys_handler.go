package publicapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/apikey"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/httpx"
)

// APIKeysHandler manages the caller's public-API tokens; Create shows the plaintext once.
type APIKeysHandler struct {
	repo *repos.APIKeyRepository
}

func NewAPIKeysHandler(repo *repos.APIKeyRepository) *APIKeysHandler {
	return &APIKeysHandler{repo: repo}
}

type createKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

type createKeyResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	Token     string `json:"token"`
	CreatedAt string `json:"created_at"`
}

type keySummary struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	Revoked    bool    `json:"revoked"`
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

	token, prefix, hash, err := apikey.Generate()
	if err != nil {
		httpx.Internal(c, err)
		return
	}

	k := &models.APIKey{
		UserID: userID,
		Name:   req.Name,
		Prefix: prefix,
		Hash:   hash,
	}
	if err := h.repo.Create(c.Request.Context(), k); err != nil {
		httpx.Internal(c, err)
		return
	}

	c.JSON(http.StatusCreated, createKeyResponse{
		ID:        k.ID.Hex(),
		Name:      k.Name,
		Prefix:    k.Prefix,
		Token:     token,
		CreatedAt: k.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *APIKeysHandler) List(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	keys, err := h.repo.ListByUser(c.Request.Context(), userID)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	out := make([]keySummary, 0, len(keys))
	for _, k := range keys {
		s := keySummary{
			ID:        k.ID.Hex(),
			Name:      k.Name,
			Prefix:    k.Prefix,
			CreatedAt: k.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			Revoked:   k.RevokedAt != nil,
		}
		if k.LastUsedAt != nil {
			t := k.LastUsedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
			s.LastUsedAt = &t
		}
		out = append(out, s)
	}
	c.JSON(http.StatusOK, gin.H{"api_keys": out})
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
