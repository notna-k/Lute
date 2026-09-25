package websocket

import (
	"log/slog"
	"time"

	"github.com/lute/api/internal/config"

	"github.com/gorilla/websocket"
)

const (
	maxMessageSize = 512 * 1024
)

type Client struct {
	hub *Hub

	conn *websocket.Conn

	send chan []byte

	userID string
}

func NewClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
	}
}

func (c *Client) ReadPump(cfg *config.WebSocketConfig) {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	_ = c.conn.SetReadDeadline(time.Now().Add(cfg.PongWait))
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(cfg.PongWait))
		return nil
	})

	for {
		// Inbound messages are discarded: relaying would let a client forge job events for
		// everyone. Reading still processes pongs and close frames.
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("websocket read", "user_id", c.userID, "err", err)
			}
			break
		}
	}
}

func (c *Client) WritePump(cfg *config.WebSocketConfig) {
	ticker := time.NewTicker(cfg.PingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(cfg.WriteWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(cfg.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) Serve(cfg *config.WebSocketConfig) {
	go c.WritePump(cfg)
	c.ReadPump(cfg)
}
