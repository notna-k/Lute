package websocket

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lute/api/internal/auth"
	"github.com/lute/api/internal/config"

	"github.com/gin-gonic/gin"
	gorillaWS "github.com/gorilla/websocket"
)

// bearerSubprotocol carries a browser's token as ["lute.bearer", "<jwt>"]: the WebSocket
// constructor cannot set a header, and ?token= would end up in access logs.
const bearerSubprotocol = "lute.bearer"

var errNoToken = errors.New("no access token")

type WebSocketHandler struct {
	hub      *Hub
	cfg      *config.Config
	tokens   *auth.TokenService
	upgrader gorillaWS.Upgrader
}

func NewWebSocketHandler(hub *Hub, cfg *config.Config, tokens *auth.TokenService) *WebSocketHandler {
	upgrader := gorillaWS.Upgrader{
		ReadBufferSize:  cfg.WebSocket.ReadBufferSize,
		WriteBufferSize: cfg.WebSocket.WriteBufferSize,
		Subprotocols:    []string{bearerSubprotocol},
		CheckOrigin: func(r *http.Request) bool {
			if !cfg.WebSocket.CheckOrigin {
				return true
			}
			return originAllowed(r.Header.Get("Origin"), cfg.Server.AllowedOrigins)
		},
	}

	return &WebSocketHandler{
		hub:      hub,
		cfg:      cfg,
		tokens:   tokens,
		upgrader: upgrader,
	}
}

func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	// Authenticate before upgrading: the hub broadcasts every build's resolved parameters.
	claims, err := h.authenticate(c.Request)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid access token"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade has already written the error response.
		slog.Warn("websocket upgrade", "err", err)
		return
	}

	client := NewClient(h.hub, conn, claims.UserID)
	h.hub.Register(client)

	client.Serve(&h.cfg.WebSocket)
}

func (h *WebSocketHandler) authenticate(r *http.Request) (*auth.AccessClaims, error) {
	if h.tokens == nil {
		return nil, errors.New("no token service configured")
	}
	raw := bearerToken(r.Header.Get("Authorization"))
	if raw == "" {
		raw = subprotocolToken(gorillaWS.Subprotocols(r))
	}
	if raw == "" {
		return nil, errNoToken
	}
	return h.tokens.ParseAccess(raw)
}

func bearerToken(header string) string {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func subprotocolToken(protocols []string) string {
	for i, p := range protocols {
		if p == bearerSubprotocol && i+1 < len(protocols) {
			return strings.TrimSpace(protocols[i+1])
		}
	}
	return ""
}

// originAllowed checks Origin against the CORS allow-list. An absent Origin passes: only
// browsers send it and every client still needs a token. "*" turns the check off.
func originAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return true
	}
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}
