package worker

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/httpx"
	"github.com/lute/api/internal/workerauth"
)

type createTokenRequest struct {
	Name string `json:"name" binding:"required"`
}

type createTokenResponse struct {
	*models.RegistrationToken
	// Token is the plaintext, shown once.
	Token string `json:"token"`
}

func (h *WorkerHandler) ListTokens(c *gin.Context) {
	tokens, err := h.tokenRepo.List(c.Request.Context())
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tokens": tokens})
}

func (h *WorkerHandler) CreateToken(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	var req createTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		httpx.Invalid(c, "name is required", map[string]string{"name": "required"})
		return
	}
	token, err := workerauth.NewToken()
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	t := &models.RegistrationToken{
		Name:      strings.TrimSpace(req.Name),
		Prefix:    workerauth.Display(token),
		TokenHash: workerauth.Hash(token),
		CreatedBy: userID,
	}
	if err := h.tokenRepo.Create(c.Request.Context(), t); err != nil {
		httpx.Internal(c, err)
		return
	}
	c.JSON(http.StatusCreated, createTokenResponse{RegistrationToken: t, Token: token})
}

// RevokeToken stops new registrations; workers that already registered keep working.
func (h *WorkerHandler) RevokeToken(c *gin.Context) {
	tokenID, err := id.FromHex(c.Param("tokenId"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid token id")
		return
	}
	if err := h.tokenRepo.Revoke(c.Request.Context(), tokenID); err != nil {
		if errors.Is(err, repos.ErrNotFound) {
			httpx.Error(c, http.StatusNotFound, "token not found")
			return
		}
		httpx.Internal(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// InstallInfo is what the Add Worker dialog needs to print a working `docker run`.
func (h *WorkerHandler) InstallInfo(c *gin.Context) {
	server := h.cfg.Workers.PublicGRPCAddr
	if server == "" {
		host := c.Request.Host
		if hostOnly, _, err := net.SplitHostPort(host); err == nil {
			host = hostOnly
		}
		if host == "" {
			host = "localhost"
		}
		server = net.JoinHostPort(host, h.cfg.GRPC.Port)
	}
	c.JSON(http.StatusOK, gin.H{"server": server, "image": h.grpcServer.WorkerImage()})
}
