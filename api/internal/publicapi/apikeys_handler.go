package publicapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/lute/api/internal/apikey"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
)

// APIKeysHandler manages issuance and revocation of public-API tokens for the
// authenticated UI user. Create returns the plaintext token exactly once.
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
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	var req createKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	token, prefix, hash, err := apikey.Generate()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	k := &models.APIKey{
		UserID: userID,
		Name:   req.Name,
		Prefix: prefix,
		Hash:   hash,
	}
	if err := h.repo.Create(c.Request.Context(), k); err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
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
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	keys, err := h.repo.ListByUser(c.Request.Context(), userID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
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
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "invalid id")
		return
	}
	if err := h.repo.Revoke(c.Request.Context(), id, userID); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			writeError(c, http.StatusNotFound, "not_found", "API key not found")
			return
		}
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
