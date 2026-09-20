package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/repos"
)

const RefreshCookieName = "lute_refresh"

// CookieConfig controls how the refresh cookie is set on responses.
type CookieConfig struct {
	Path     string
	Domain   string
	Secure   bool
	SameSite http.SameSite
}

// DefaultCookieConfig returns sane defaults: scoped to /api/v1/auth so the
// cookie is only sent to refresh / logout. SameSite=Strict blocks CSRF.
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	tokens, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, sessionMeta(c))
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		return
	}
	h.setRefreshCookie(c, tokens.RefreshPlaintext, tokens.RefreshExpiresAt)
	c.JSON(http.StatusOK, tokenResponseFrom(tokens))
}

func (h *Handler) Refresh(c *gin.Context) {
	raw, err := c.Cookie(RefreshCookieName)
	if err != nil || raw == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh token"})
		return
	}
	tokens, err := h.svc.Refresh(c.Request.Context(), raw, sessionMeta(c))
	if err != nil {
		// Clear the cookie on any failure so the client stops sending it.
		h.clearRefreshCookie(c)
		if errors.Is(err, ErrTokenReuse) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "session revoked"})
			return
		}
		if errors.Is(err, ErrInvalidToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "refresh failed"})
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

// Me returns the authenticated user. Must be mounted under the JWT middleware.
func (h *Handler) Me(c *gin.Context) {
	uidStr, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	uid, err := id.FromHex(uidStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id"})
		return
	}
	user, err := h.users.GetByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
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
