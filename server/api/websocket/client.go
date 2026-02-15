package websocket

import (
	"log"
	"time"

	"github.com/lute/api/config"

	"github.com/gorilla/websocket"
)

const (
	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024
)

// Client is a middleman between the websocket connection and the hub
type Client struct {
	hub *Hub

	// The websocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// User ID associated with this client
	userID string
}

// NewClient creates a new client instance
func NewClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) ReadPump(cfg *config.WebSocketConfig) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(cfg.PongWait))
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(cfg.PongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle incoming message
		log.Printf("Received message from client %s: %s", c.userID, string(message))

		// Echo message back (you can implement custom message handling here)
		c.hub.broadcast <- message
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) WritePump(cfg *config.WebSocketConfig) {
	ticker := time.NewTicker(cfg.PingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(cfg.WriteWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(cfg.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Serve starts the client's read and write pumps
func (c *Client) Serve(cfg *config.WebSocketConfig) {
	// Start read and write pumps
	go c.WritePump(cfg)
	c.ReadPump(cfg)
}
