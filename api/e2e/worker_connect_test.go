//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/lute/api/e2e/harness"
)

// TestAgentConnection covers what core does with an agent's stream: accept it, describe
// it to the panel, refuse the ones it should, and notice when it goes away.
func TestAgentConnection(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()

	t.Run("Success - a connected agent advertises its queues and capacity", func(t *testing.T) {
		reg := stack.ClaimWorker(admin, "connect-basic")
		stack.StartAgent(reg.WorkerID, harness.WithQueues("build", "deploy"), harness.WithConcurrency(3))

		got := stack.WaitConnected(admin, reg.WorkerID)
		want := harness.ConnectedWorker{
			WorkerID:    reg.WorkerID,
			Queues:      []string{"build", "deploy"},
			Concurrency: 3,
			ActiveJobs:  0,
			Draining:    false,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("connected worker mismatch (-want +got):\n%s", diff)
		}

		// The panel's worker list must agree that this host is live.
		worker, err := admin.GetWorker(reg.WorkerID)
		if err != nil {
			t.Fatalf("get worker: %v", err)
		}
		if worker.Status == "pending" {
			t.Errorf("status = pending after the agent connected; the panel would show it as never seen")
		}
	})

	t.Run("Success - an agent started against a deleted worker gives up", func(t *testing.T) {
		reg := stack.ClaimWorker(admin, "deleted-host")
		agent := stack.StartAgent(reg.WorkerID, harness.WithQueues("build"))
		stack.WaitConnected(admin, reg.WorkerID)

		if err := admin.DeleteWorker(reg.WorkerID); err != nil {
			t.Fatalf("delete worker: %v", err)
		}
		agent.WaitExit(30 * time.Second)
		stack.WaitDisconnected(admin, reg.WorkerID)

		// The interesting case is the host that comes back later — after a reboot, or
		// because something restarts it. Retrying a worker id core has forgotten is a
		// reconnect storm against a server that will never accept it, so the agent has
		// to stop rather than back off and try again.
		revenant := stack.StartAgent(reg.WorkerID, harness.WithQueues("build"))
		revenant.WaitExit(30 * time.Second)
		harness.Never(t, 2*time.Second, "a deleted worker to reappear as connected", func() bool {
			workers, err := admin.ConnectedWorkers()
			if err != nil {
				return false
			}
			for _, w := range workers {
				if w.WorkerID == reg.WorkerID {
					return true
				}
			}
			return false
		})
	})

	t.Run("Success - a dead host is refused until an operator re-enables it", func(t *testing.T) {
		// A short heartbeat is what makes a missing agent turn into a dead worker in
		// a test's lifetime rather than a deployment's.
		fast := newStack(t, harness.WithFastHeartbeat(300*time.Millisecond, time.Second, 2))
		fastAdmin := fast.AdminClient()

		reg := fast.ClaimWorker(fastAdmin, "flatlined")
		agent := fast.StartAgent(reg.WorkerID, harness.WithQueues("build"))
		fast.WaitConnected(fastAdmin, reg.WorkerID)

		agent.Kill()
		harness.WaitFor(t, 30*time.Second, "core to mark the host dead", func() bool {
			w, err := fastAdmin.GetWorker(reg.WorkerID)
			return err == nil && w.Status == "dead"
		})

		// Until an operator says otherwise, a dead host stays out: its agent may be a
		// zombie, and letting it back in silently hides a real failure.
		refused := fast.StartAgent(reg.WorkerID, harness.WithQueues("build"))
		refused.WaitExit(30 * time.Second)

		if _, err := fastAdmin.ReEnableWorker(reg.WorkerID); err != nil {
			t.Fatalf("re-enable: %v", err)
		}
		fast.StartAgent(reg.WorkerID, harness.WithQueues("build"))
		fast.WaitConnected(fastAdmin, reg.WorkerID)
	})

	t.Run("Success - a heartbeat records the host's metrics", func(t *testing.T) {
		fast := newStack(t, harness.WithFastHeartbeat(300*time.Millisecond, 2*time.Second, 5))
		fastAdmin := fast.AdminClient()

		agent := fast.ConnectedAgent(fastAdmin, "reporting-host", harness.WithQueues("build"))

		status := harness.Eventually(t, 30*time.Second, "the host to report metrics",
			func() (harness.WorkerLiveStatus, bool) {
				s, err := fastAdmin.WorkerStatus(agent.WorkerID)
				if err != nil {
					return s, false
				}
				return s, s.LastSeen != "" && len(s.Metrics) > 0
			})

		// The dashboard is only as good as these: a host reporting nothing looks
		// identical to one that is idle.
		for _, key := range []string{"cpu_load", "mem_usage_mb"} {
			if _, ok := status.Metrics[key]; !ok {
				t.Errorf("metric %q missing; got %v", key, status.Metrics)
			}
		}
	})

	t.Run("Success - deleting a live host stops its agent", func(t *testing.T) {
		agent := stack.ConnectedAgent(admin, "stop-on-delete", harness.WithQueues("build"))

		if err := admin.DeleteWorker(agent.WorkerID); err != nil {
			t.Fatalf("delete worker: %v", err)
		}
		// Leaving the agent running would keep a deleted host claiming work.
		agent.WaitExit(30 * time.Second)

		if _, err := admin.GetWorker(agent.WorkerID); harness.StatusOf(err) != 404 {
			t.Errorf("get deleted worker: err = %v, want 404", err)
		}
	})

	t.Run("Success - an agent reconnects by itself after core restarts", func(t *testing.T) {
		agent := stack.ConnectedAgent(admin, "survives-restart", harness.WithQueues("build"))

		stack.Restart()
		restarted := stack.AdminClient()

		// A core deploy must not need a visit to every build host.
		stack.WaitConnected(restarted, agent.WorkerID)
		if agent.Exited() {
			t.Fatalf("the agent gave up when core restarted (exit: %v):\n%s", agent.ExitError(), agent.Stderr())
		}

		// And the host must be able to work again, not just be connected.
		build, err := restarted.Trigger(echoSlug, map[string]any{"environment": "staging"})
		if err != nil {
			t.Fatalf("trigger after restart: %v", err)
		}
		harness.WaitBuildStatus(restarted, echoSlug, build.RunID, 2*time.Minute, "passed")
	})
}

// TestConnectionRejections covers the protocol's front door, using a client that can
// misbehave in ways the real agent never does.
func TestConnectionRejections(t *testing.T) {
	stack := newBareStack(t)
	admin := stack.AdminClient()

	t.Run("Fail - an unknown worker id is refused", func(t *testing.T) {
		raw, err := stack.DialRawWorker("0123456789abcdef01234567")
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		streamErr := raw.WaitForStreamEnd(15 * time.Second)
		if !strings.Contains(streamErr.Error(), "not found") {
			t.Errorf("stream ended with %v, want a not-found refusal", streamErr)
		}
	})

	t.Run("Fail - a malformed worker id is refused", func(t *testing.T) {
		raw, err := stack.DialRawWorker("not-a-worker-id")
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if streamErr := raw.WaitForStreamEnd(15 * time.Second); streamErr == nil {
			t.Error("core accepted a malformed worker id")
		}
	})

	t.Run("Success - the newest stream for a worker is the one that gets work", func(t *testing.T) {
		reg := stack.ClaimWorker(admin, "double-agent")

		first, err := stack.DialRawWorker(reg.WorkerID)
		if err != nil {
			t.Fatalf("dial first: %v", err)
		}
		if err := first.Register(1, "build"); err != nil {
			t.Fatalf("register first: %v", err)
		}
		stack.WaitConnected(admin, reg.WorkerID)

		second, err := stack.DialRawWorker(reg.WorkerID)
		if err != nil {
			t.Fatalf("dial second: %v", err)
		}
		if err := second.Register(1, "build"); err != nil {
			t.Fatalf("register second: %v", err)
		}
		harness.WaitFor(t, 15*time.Second, "core to see the second stream's registration", func() bool {
			workers, err := admin.ConnectedWorkers()
			if err != nil {
				return false
			}
			for _, w := range workers {
				if w.WorkerID == reg.WorkerID && len(w.Queues) > 0 {
					return true
				}
			}
			return false
		})

		// One host, one stream: work must not be handed to a connection that was
		// replaced, or it vanishes into a socket nobody is reading.
		enqueued, err := admin.Enqueue(harness.EnqueueRequest{Queue: "build", Type: "noop"})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		assignment := second.WaitForAssignment(30 * time.Second)
		if assignment.JobId != enqueued.JobID {
			t.Errorf("assignment job id = %s, want %s", assignment.JobId, enqueued.JobID)
		}
		if got := first.Assignments(); len(got) != 0 {
			t.Errorf("the replaced stream received %d assignment(s)", len(got))
		}
	})
}
