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

// panelOrigin is on core's allow-list, so an event stream opens as the panel's does.
const panelOrigin = "http://localhost:8080"

type Event struct {
	Type string          `json:"type"`
	Job  json.RawMessage `json:"job"`
}

func (e Event) JobID() string {
	var job struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(e.Job, &job)
	return job.ID
}

type EventStream struct {
	t    *testing.T
	conn *gorillaWS.Conn

	mu     sync.Mutex
	events []Event
	err    error
}

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

// DialEventStream returns the raw upgrade response, for tests that expect a refusal.
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

func (es *EventStream) Events() []Event {
	es.mu.Lock()
	defer es.mu.Unlock()
	return append([]Event(nil), es.events...)
}

func (es *EventStream) TypesFor(jobID string) []string {
	var out []string
	for _, ev := range es.Events() {
		if ev.JobID() == jobID {
			out = append(out, ev.Type)
		}
	}
	return out
}

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

// Send writes a raw frame. Reads belong to the collector goroutine; check Events instead.
func (es *EventStream) Send(payload string) error {
	return es.conn.WriteMessage(gorillaWS.TextMessage, []byte(payload))
}
