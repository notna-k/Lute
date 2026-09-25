//go:build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/lute/api/e2e/harness"
)

// TestBuildEventStream covers the live view: what the panel is told, as it happens,
// without polling for it.
func TestBuildEventStream(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()
	stack.ConnectedAgent(admin, "event-host", harness.WithQueues("build"))

	t.Run("Success - a build reports started then completed", func(t *testing.T) {
		events := stack.OpenEventStream(admin)

		build, err := admin.Trigger(echoSlug, map[string]any{"environment": "staging"})
		if err != nil {
			t.Fatalf("trigger: %v", err)
		}
		jobID := jobIDOf(t, admin, echoSlug, build.RunID)

		events.WaitForEvent(2*time.Minute, jobID, "job_completed")

		// Order matters: a panel that sees "completed" before "started" draws a build
		// that finished before it began.
		got := events.TypesFor(jobID)
		if diff := cmp.Diff([]string{"job_started", "job_completed"}, got); diff != "" {
			t.Errorf("event sequence mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Success - a failing build reports failed", func(t *testing.T) {
		events := stack.OpenEventStream(admin)

		build, err := admin.Trigger(failingSlug, map[string]any{})
		if err != nil {
			t.Fatalf("trigger: %v", err)
		}
		jobID := jobIDOf(t, admin, failingSlug, build.RunID)

		events.WaitForEvent(3*time.Minute, jobID, "job_failed")
	})

	t.Run("Fail - an anonymous client cannot join the stream", func(t *testing.T) {
		// Every build's resolved parameters go over this socket.
		_, resp, err := stack.DialEventStream(http.Header{"Origin": {"http://localhost:8080"}})
		if err == nil {
			t.Fatal("an anonymous client completed the upgrade")
		}
		if resp == nil || resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %v, want 401", resp)
		}
	})

	t.Run("Fail - a client cannot inject events for other sessions", func(t *testing.T) {
		listener := stack.OpenEventStream(admin)
		sender := stack.OpenEventStream(admin)

		// Relaying this would let any signed-in session forge a green build for
		// everyone else's panel.
		if err := sender.Send(`{"type":"job_completed","job":{"id":"forged-build"}}`); err != nil {
			t.Fatalf("send: %v", err)
		}

		harness.Never(t, time.Second, "a client's message to be relayed to another client", func() bool {
			return len(listener.TypesFor("forged-build")) > 0
		})
	})
}
