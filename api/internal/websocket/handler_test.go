package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gorillaWS "github.com/gorilla/websocket"

	"github.com/lute/api/internal/auth"
	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/id"
)

const testSecret = "test-secret-that-is-long-enough-32"

func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			AllowedOrigins: []string{"http://localhost:8080"},
		},
		WebSocket: config.WebSocketConfig{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     true,
			PingPeriod:      54 * time.Second,
			PongWait:        60 * time.Second,
			WriteWait:       10 * time.Second,
		},
	}
}

// newTestServer serves the WS route as the router mounts it, with a valid token for it.
func newTestServer(t *testing.T) (wsURL, token string, hub *Hub) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tokens, err := auth.NewTokenService(testSecret, time.Minute, time.Hour, "lute")
	if err != nil {
		t.Fatalf("token service: %v", err)
	}
	signed, _, err := tokens.SignAccess(id.New(), "admin@example.com")
	if err != nil {
		t.Fatalf("sign access: %v", err)
	}

	hub = NewHub()
	go hub.Run()

	h := NewWebSocketHandler(hub, testConfig(), tokens)
	r := gin.New()
	r.GET("/api/ws", h.HandleWebSocket)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws", signed, hub
}

func dial(t *testing.T, wsURL string, header http.Header, subprotocols []string) (*gorillaWS.Conn, *http.Response, error) {
	t.Helper()
	dialer := gorillaWS.Dialer{
		HandshakeTimeout: 5 * time.Second,
		Subprotocols:     subprotocols,
	}
	conn, resp, err := dialer.Dial(wsURL, header)
	if conn != nil {
		t.Cleanup(func() { _ = conn.Close() })
	}
	return conn, resp, err
}

func TestUpgradeWithoutTokenIsRejected(t *testing.T) {
	wsURL, _, hub := newTestServer(t)

	// The hub carries every build's parameters; an anonymous caller must not join it.
	_, resp, err := dial(t, wsURL, http.Header{"Origin": {"http://localhost:8080"}}, nil)
	if err == nil {
		t.Fatal("anonymous client completed the upgrade")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
	if n := hub.GetClientCount(); n != 0 {
		t.Fatalf("hub has %d clients, want 0", n)
	}
}

func TestUpgradeWithInvalidTokenIsRejected(t *testing.T) {
	wsURL, token, _ := newTestServer(t)

	// Same token, signed with a different secret: a forgery must not pass.
	other, err := auth.NewTokenService("another-secret-that-is-long-enough", time.Minute, time.Hour, "lute")
	if err != nil {
		t.Fatalf("token service: %v", err)
	}
	forged, _, err := other.SignAccess(id.New(), "attacker@example.com")
	if err != nil {
		t.Fatalf("sign access: %v", err)
	}
	if forged == token {
		t.Fatal("forged token matched the real one")
	}

	_, resp, err := dial(t, wsURL, http.Header{
		"Origin":        {"http://localhost:8080"},
		"Authorization": {"Bearer " + forged},
	}, nil)
	if err == nil {
		t.Fatal("client with a forged token completed the upgrade")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

func TestUpgradeWithBearerHeaderSucceeds(t *testing.T) {
	wsURL, token, _ := newTestServer(t)

	conn, _, err := dial(t, wsURL, http.Header{
		"Origin":        {"http://localhost:8080"},
		"Authorization": {"Bearer " + token},
	}, nil)
	if err != nil {
		t.Fatalf("authenticated upgrade failed: %v", err)
	}
	if conn == nil {
		t.Fatal("no connection")
	}
}

func TestUpgradeWithBearerSubprotocolSucceeds(t *testing.T) {
	wsURL, token, _ := newTestServer(t)

	// A browser cannot set a header, so it sends the token after the bearer marker.
	conn, _, err := dial(t, wsURL, http.Header{"Origin": {"http://localhost:8080"}},
		[]string{bearerSubprotocol, token})
	if err != nil {
		t.Fatalf("subprotocol upgrade failed: %v", err)
	}
	if got := conn.Subprotocol(); got != bearerSubprotocol {
		t.Fatalf("negotiated subprotocol = %q, want %q", got, bearerSubprotocol)
	}
}

func TestClientMessagesAreNotRelayedToOtherClients(t *testing.T) {
	wsURL, token, hub := newTestServer(t)
	header := http.Header{
		"Origin":        {"http://localhost:8080"},
		"Authorization": {"Bearer " + token},
	}

	sender, _, err := dial(t, wsURL, header, nil)
	if err != nil {
		t.Fatalf("dial sender: %v", err)
	}
	listener, _, err := dial(t, wsURL, header, nil)
	if err != nil {
		t.Fatalf("dial listener: %v", err)
	}
	waitForClients(t, hub, 2)

	// Relaying this would let any panel session forge a build result for every other.
	forged := []byte(`{"type":"job_completed","job":{"id":"not-a-real-job"}}`)
	if err := sender.WriteMessage(gorillaWS.TextMessage, forged); err != nil {
		t.Fatalf("write: %v", err)
	}

	_ = listener.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
	if _, msg, err := listener.ReadMessage(); err == nil {
		t.Fatalf("client message was relayed to another client: %s", msg)
	}
}

func TestCoreBroadcastsReachConnectedClients(t *testing.T) {
	wsURL, token, hub := newTestServer(t)

	conn, _, err := dial(t, wsURL, http.Header{
		"Origin":        {"http://localhost:8080"},
		"Authorization": {"Bearer " + token},
	}, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	waitForClients(t, hub, 1)

	hub.Broadcast([]byte(`{"type":"job_started"}`))

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("broadcast did not reach the client: %v", err)
	}
	if string(msg) != `{"type":"job_started"}` {
		t.Fatalf("got %s, want the broadcast", msg)
	}
}

func waitForClients(t *testing.T, hub *Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hub.GetClientCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("hub has %d clients, want %d", hub.GetClientCount(), want)
}

func TestUpgradeFromUnlistedOriginIsRejected(t *testing.T) {
	wsURL, token, _ := newTestServer(t)

	_, resp, err := dial(t, wsURL, http.Header{
		"Origin":        {"http://evil.example.com"},
		"Authorization": {"Bearer " + token},
	}, nil)
	if err == nil {
		t.Fatal("upgrade from an unlisted origin succeeded")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %v, want 403", resp)
	}
}

func TestOriginAllowed(t *testing.T) {
	allowed := []string{"http://localhost:8080", "https://lute.example.com"}

	cases := []struct {
		name   string
		origin string
		list   []string
		want   bool
	}{
		{"listed origin", "http://localhost:8080", allowed, true},
		{"listed origin, different case", "HTTP://LOCALHOST:8080", allowed, true},
		{"unlisted origin", "http://evil.example.com", allowed, false},
		{"scheme must match", "https://localhost:8080", allowed, false},
		// Only browsers send Origin, and every caller still needs a token.
		{"absent origin", "", allowed, true},
		{"wildcard turns the check off", "http://evil.example.com", []string{"*"}, true},
		{"empty allow-list rejects browsers", "http://localhost:8080", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := originAllowed(tc.origin, tc.list); got != tc.want {
				t.Fatalf("originAllowed(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}

func TestSubprotocolToken(t *testing.T) {
	cases := []struct {
		name      string
		protocols []string
		want      string
	}{
		{"marker then token", []string{bearerSubprotocol, "abc"}, "abc"},
		{"marker with nothing after it", []string{bearerSubprotocol}, ""},
		{"no marker", []string{"abc"}, ""},
		{"none offered", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := subprotocolToken(tc.protocols); got != tc.want {
				t.Fatalf("subprotocolToken(%v) = %q, want %q", tc.protocols, got, tc.want)
			}
		})
	}
}

func TestBearerToken(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"Bearer abc", "abc"},
		{"bearer abc", "abc"},
		{"Bearer  abc ", "abc"},
		{"Basic abc", ""},
		{"abc", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := bearerToken(tc.header); got != tc.want {
			t.Fatalf("bearerToken(%q) = %q, want %q", tc.header, got, tc.want)
		}
	}
}
