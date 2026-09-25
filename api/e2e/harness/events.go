//go:build e2e

package harness

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	gorillaWS "github.com/gorilla/websocket"
)

// panelOrigin is an origin core's allow-list accepts, so an event stream opens the
// way the panel's does.
const panelOrigin = "http://localhost:8080"

// Event is one message core pushed to the panel.
type Event struct {
	Type string          `json:"type"`
	Job  json.RawMessage `json:"job"`
}

// JobID pulls the job identifier out of an event's payload.
func (e Event) JobID() string {
	var job struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(e.Job, &job)
	return job.ID
}

// EventStream is a panel session listening for build events.
type EventStream struct {
	t    *testing.T
	conn *gorillaWS.Conn

	mu     sync.Mutex
	events []Event
	err    error
}

// OpenEventStream connects to the panel's WebSocket with the client's token.
func (s *Stack) OpenEventStream(c *Client) *EventStream {
	s.t.Helper()

	url := "ws" + strings.TrimPrefix(s.BaseURL(), "http") + "/api/ws"
	dialer := gorillaWS.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, resp, err := dialer.Dial(url, http.Header{
		"Origin":        {panelOrigin},
		"Authorization": {"Bearer " + c.Token()},
	})
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		s.t.Fatalf("open event stream: %v (status %d)", err, status)
	}

	es := &EventStream{t: s.t, conn: conn}
	go es.read()
	s.t.Cleanup(func() { _ = conn.Close() })
	return es
}

// DialEventStream attempts the upgrade and returns the response, for tests that
// assert an upgrade is refused.
func (s *Stack) DialEventStream(header http.Header) (*gorillaWS.Conn, *http.Response, error) {
	s.t.Helper()
	url := "ws" + strings.TrimPrefix(s.BaseURL(), "http") + "/api/ws"
	dialer := gorillaWS.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, resp, err := dialer.Dial(url, header)
	if conn != nil {
		s.t.Cleanup(func() { _ = conn.Close() })
	}
	return conn, resp, err
}

func (es *EventStream) read() {
	for {
		_, raw, err := es.conn.ReadMessage()
		if err != nil {
			es.mu.Lock()
			es.err = err
			es.mu.Unlock()
			return
		}
		var ev Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			continue
		}
		es.mu.Lock()
		es.events = append(es.events, ev)
		es.mu.Unlock()
	}
}

// Events returns everything received so far.
func (es *EventStream) Events() []Event {
	es.mu.Lock()
	defer es.mu.Unlock()
	return append([]Event(nil), es.events...)
}

// TypesFor returns the event types seen for one job, in arrival order.
func (es *EventStream) TypesFor(jobID string) []string {
	var out []string
	for _, ev := range es.Events() {
		if ev.JobID() == jobID {
			out = append(out, ev.Type)
		}
	}
	return out
}

// WaitForEvent blocks until an event of the given type arrives for a job.
func (es *EventStream) WaitForEvent(timeout time.Duration, jobID, eventType string) Event {
	es.t.Helper()
	return Eventually(es.t, timeout, "event "+eventType+" for job "+jobID, func() (Event, bool) {
		for _, ev := range es.Events() {
			if ev.Type == eventType && ev.JobID() == jobID {
				return ev, true
			}
		}
		return Event{}, false
	})
}

// Send writes a raw frame. Reads stay with the collector goroutine, so a test
// asserting something never arrives checks Events rather than reading itself.
func (es *EventStream) Send(payload string) error {
	return es.conn.WriteMessage(gorillaWS.TextMessage, []byte(payload))
}
