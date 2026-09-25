package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/httpx"
)

const RefreshCookieName = "lute_refresh"

type CookieConfig struct {
	Path     string
	Domain   string
	Secure   bool
	SameSite http.SameSite
}

// DefaultCookieConfig scopes the cookie to /api/v1/auth; SameSite=Strict blocks CSRF.
func DefaultCookieConfig(secure bool) CookieConfig {
	return CookieConfig{
		Path:     "/api/v1/auth",
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}
}

type Handler struct {
	svc    *Service
	users  *repos.UserRepository
	cookie CookieConfig
}

func NewHandler(svc *Service, users *repos.UserRepository, cookie CookieConfig) *Handler {
	return &Handler{svc: svc, users: users, cookie: cookie}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userDTO struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type tokenResponse struct {
	AccessToken string  `json:"access_token"`
	ExpiresIn   int64   `json:"expires_in"`
	TokenType   string  `json:"token_type"`
	User        userDTO `json:"user"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid request body")
		return
	}
	tokens, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, sessionMeta(c))
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			httpx.Error(c, http.StatusUnauthorized, "invalid email or password")
			return
		}
		httpx.Internal(c, fmt.Errorf("login: %w", err))
		return
	}
	h.setRefreshCookie(c, tokens.RefreshPlaintext, tokens.RefreshExpiresAt)
	c.JSON(http.StatusOK, tokenResponseFrom(tokens))
}

func (h *Handler) Refresh(c *gin.Context) {
	raw, err := c.Cookie(RefreshCookieName)
	if err != nil || raw == "" {
		httpx.Error(c, http.StatusUnauthorized, "missing refresh token")
		return
	}
	tokens, err := h.svc.Refresh(c.Request.Context(), raw, sessionMeta(c))
	if err != nil {
		h.clearRefreshCookie(c)
		if errors.Is(err, ErrTokenReuse) {
			httpx.Error(c, http.StatusUnauthorized, "session revoked")
			return
		}
		if errors.Is(err, ErrInvalidToken) {
			httpx.Error(c, http.StatusUnauthorized, "invalid refresh token")
			return
		}
		httpx.Internal(c, fmt.Errorf("refresh: %w", err))
		return
	}
	h.setRefreshCookie(c, tokens.RefreshPlaintext, tokens.RefreshExpiresAt)
	c.JSON(http.StatusOK, tokenResponseFrom(tokens))
}

func (h *Handler) Logout(c *gin.Context) {
	if raw, err := c.Cookie(RefreshCookieName); err == nil && raw != "" {
		_ = h.svc.Logout(c.Request.Context(), raw)
	}
	h.clearRefreshCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Me must be mounted under the JWT middleware.
func (h *Handler) Me(c *gin.Context) {
	uid, ok := httpx.UserID(c)
	if !ok {
		return
	}
	user, err := h.users.GetByID(c.Request.Context(), uid)
	if err != nil {
		httpx.NotFoundOrInternal(c, err, "user not found")
		return
	}
	c.JSON(http.StatusOK, userDTO{
		ID:          user.ID.Hex(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
	})
}

func tokenResponseFrom(t *IssuedTokens) tokenResponse {
	return tokenResponse{
		AccessToken: t.Access,
		ExpiresIn:   int64(time.Until(t.AccessExpiresAt).Seconds()),
		TokenType:   "Bearer",
		User: userDTO{
			ID:          t.User.ID.Hex(),
			Email:       t.User.Email,
			DisplayName: t.User.DisplayName,
		},
	}
}

func (h *Handler) setRefreshCookie(c *gin.Context, value string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	c.SetSameSite(h.cookie.SameSite)
	c.SetCookie(RefreshCookieName, value, maxAge, h.cookie.Path, h.cookie.Domain, h.cookie.Secure, true)
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(h.cookie.SameSite)
	c.SetCookie(RefreshCookieName, "", -1, h.cookie.Path, h.cookie.Domain, h.cookie.Secure, true)
}

func sessionMeta(c *gin.Context) SessionMeta {
	return SessionMeta{
		UserAgent: strings.TrimSpace(c.Request.UserAgent()),
		IP:        c.ClientIP(),
	}
}
