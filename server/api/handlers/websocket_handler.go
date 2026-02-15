package handlers

import (
	"log"
	"net/http"

	"github.com/lute/api/config"
	"github.com/lute/api/websocket"

	"github.com/gin-gonic/gin"
	gorillaWS "github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	hub      *websocket.Hub
	cfg      *config.Config
	upgrader gorillaWS.Upgrader
}

func NewWebSocketHandler(hub *websocket.Hub, cfg *config.Config) *WebSocketHandler {
	upgrader := gorillaWS.Upgrader{
		ReadBufferSize:  cfg.WebSocket.ReadBufferSize,
		WriteBufferSize: cfg.WebSocket.WriteBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			if !cfg.WebSocket.CheckOrigin {
				return true
			}
			// TODO: Implement proper origin checking
			return true
		},
	}

	return &WebSocketHandler{
		hub:      hub,
		cfg:      cfg,
		upgrader: upgrader,
	}
}

// HandleWebSocket handles WebSocket connections
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to upgrade connection"})
		return
	}

	// Get user ID from context (set by auth middleware)
	userID := "anonymous"
	if uid, exists := c.Get("user_id"); exists {
		userID = uid.(string)
	}

	// Create and register client
	client := websocket.NewClient(h.hub, conn, userID)
	h.hub.Register(client)

	// Start serving the client
	client.Serve(&h.cfg.WebSocket)
}
