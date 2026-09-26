//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lute/api/e2e/harness"
)

// TestAgentConnection covers accepting, describing, refusing and losing an agent's stream.
func TestAgentConnection(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()

	t.Run("Success - a connected agent advertises its queues and capacity", func(t *testing.T) {
		agent := stack.ConnectedAgent(admin, "connect-basic", harness.WithQueues("build", "deploy"), harness.WithConcurrency(3))

		got := stack.WaitConnected(admin, agent.WorkerID)
		want := harness.ConnectedWorker{
			WorkerID:    agent.WorkerID,
			Queues:      []string{"build", "deploy"},
			Concurrency: 3,
			ActiveJobs:  0,
			Draining:    false,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("connected worker mismatch (-want +got):\n%s", diff)
		}

		worker, err := admin.GetWorker(agent.WorkerID)
		if err != nil {
			t.Fatalf("get worker: %v", err)
		}
		if worker.Status == "pending" {
			t.Errorf("status = pending after the agent connected; the panel would show it as never seen")
		}
	})

	t.Run("Success - a worker deleted while offline stops when it comes back", func(t *testing.T) {
		agent := stack.ConnectedAgent(admin, "deleted-offline", harness.WithQueues("build"))
		agent.Kill()
		stack.WaitDisconnected(admin, agent.WorkerID)

		result, err := admin.DeleteWorker(agent.WorkerID)
		if err != nil {
			t.Fatalf("delete worker: %v", err)
		}
		if result != "deleted" {
			t.Errorf("delete result = %q, want deleted: nothing is connected to drain", result)
		}

		// A host coming back with an id core has forgotten must stop, not retry forever.
		revenant := agent.Restart()
		revenant.WaitExit(30 * time.Second)
		if revenant.ExitCode() != 0 {
			t.Errorf("exit code = %d, want 0 so a container stays stopped:\n%s", revenant.ExitCode(), revenant.Stderr())
		}
		if !strings.Contains(revenant.Stderr(), "remove state.json") {
			t.Errorf("the agent did not say how to register again:\n%s", revenant.Stderr())
		}
		harness.Never(t, 2*time.Second, "a deleted worker to reappear as connected", func() bool {
			workers, err := admin.ConnectedWorkers()
			if err != nil {
				return false
			}
			for _, w := range workers {
				if w.WorkerID == agent.WorkerID {
					return true
				}
			}
			return false
		})
	})

	t.Run("Success - a dead host is refused until an operator re-enables it", func(t *testing.T) {
		fast := newStack(t, harness.WithFastHeartbeat(300*time.Millisecond, time.Second, 2))
		fastAdmin := fast.AdminClient()

		agent := fast.ConnectedAgent(fastAdmin, "flatlined", harness.WithQueues("build"))

		agent.Kill()
		harness.WaitFor(t, 30*time.Second, "core to mark the host dead", func() bool {
			w, err := fastAdmin.GetWorker(agent.WorkerID)
			return err == nil && w.Status == "dead"
		})

		// A dead host stays out until re-enabled: letting a zombie back in hides a real failure.
		refused := agent.Restart()
		refused.WaitExit(30 * time.Second)
		if refused.ExitCode() != 78 {
			t.Errorf("exit code = %d, want 78", refused.ExitCode())
		}

		if _, err := fastAdmin.ReEnableWorker(agent.WorkerID); err != nil {
			t.Fatalf("re-enable: %v", err)
		}
		refused.Restart()
		fast.WaitConnected(fastAdmin, agent.WorkerID)
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

		// A host reporting nothing would look identical to an idle one.
		for _, key := range []string{"cpu_load", "mem_usage_mb"} {
			if _, ok := status.Metrics[key]; !ok {
				t.Errorf("metric %q missing; got %v", key, status.Metrics)
			}
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

		build, err := restarted.Trigger(echoSlug, map[string]any{"environment": "staging"})
		if err != nil {
			t.Fatalf("trigger after restart: %v", err)
		}
		harness.WaitBuildStatus(restarted, echoSlug, build.RunID, 2*time.Minute, "passed")
	})
}

// TestConnectionRejections uses a raw client to misbehave in ways the real agent never does.
func TestConnectionRejections(t *testing.T) {
	stack := newBareStack(t)
	admin := stack.AdminClient()

	t.Run("Fail - an unknown worker id is refused", func(t *testing.T) {
		raw, err := stack.DialRawWorker("0123456789abcdef01234567", "lute_ws_x")
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if code := status.Code(raw.WaitForStreamEnd(15 * time.Second)); code != codes.NotFound {
			t.Errorf("stream ended with %v, want NotFound", code)
		}
	})

	t.Run("Fail - a malformed worker id is refused", func(t *testing.T) {
		raw, err := stack.DialRawWorker("not-a-worker-id", "lute_ws_x")
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if streamErr := raw.WaitForStreamEnd(15 * time.Second); streamErr == nil {
			t.Error("core accepted a malformed worker id")
		}
	})

	t.Run("Success - the newest stream for a worker is the one that gets work", func(t *testing.T) {
		reg := stack.RegisterWorker("double-agent")

		first, err := stack.DialRawWorker(reg.WorkerID, reg.Secret)
		if err != nil {
			t.Fatalf("dial first: %v", err)
		}
		if err := first.Register(1, "build"); err != nil {
			t.Fatalf("register first: %v", err)
		}
		stack.WaitConnected(admin, reg.WorkerID)

		second, err := stack.DialRawWorker(reg.WorkerID, reg.Secret)
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

		// Work must not go to a replaced stream, or it vanishes into a socket nobody reads.
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
