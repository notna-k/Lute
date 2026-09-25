//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/lute/api/e2e/harness"
)

// TestLostHostsAndRetries covers what happens when a build host stops answering. This
// is what separates a CI server you can trust from one you cannot: a build must never
// sit in "running" because the machine that owned it went away.
func TestLostHostsAndRetries(t *testing.T) {
	t.Run("Success - a killed host's build is failed, not left running", func(t *testing.T) {
		stack := newStack(t, harness.WithLeaseGrace(2*time.Second, 2*time.Second))
		admin := stack.AdminClient()
		agent := stack.ConnectedAgent(admin, "doomed-host", harness.WithQueues("build"))

		// Queued directly rather than triggered, so the build can carry a short
		// timeout: core measures a lost build against its timeout plus the lease
		// grace, and a definition has no way to set one (they all get 300s).
		enqueued, err := admin.Enqueue(harness.EnqueueRequest{
			Queue:      "build",
			Type:       "container",
			TimeoutSec: 3,
			MaxRetries: 3,
			Payload:    containerPayload(t, "bash:5", "echo started; sleep 120"),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		harness.WaitJobStatus(admin, enqueued.JobID, time.Minute, "running")

		// The machine disappears mid-build: no result, no goodbye.
		agent.Kill()

		// Core has to notice on its own. Left alone, this build would read as running
		// forever, and its capacity would never be reclaimed.
		job := harness.WaitJobStatus(admin, enqueued.JobID, 2*time.Minute, "pending", "dead")
		if job.Attempts == 0 {
			t.Error("the lost attempt was not counted")
		}
		if job.Error == "" {
			t.Error("the lost build carries no explanation of what happened")
		}
	})

	t.Run("Success - a build another host can finish is retried there", func(t *testing.T) {
		stack := newStack(t, harness.WithLeaseGrace(2*time.Second, 2*time.Second))
		admin := stack.AdminClient()
		doomed := stack.ConnectedAgent(admin, "retry-first", harness.WithQueues("build"))

		// Long enough to still be running when its host dies, short enough that the
		// lease expires while this test is watching.
		enqueued, err := admin.Enqueue(harness.EnqueueRequest{
			Queue:      "build",
			Type:       "container",
			TimeoutSec: 3,
			MaxRetries: 3,
			Payload:    containerPayload(t, "bash:5", "echo started; sleep 20"),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		harness.WaitJobStatus(admin, enqueued.JobID, time.Minute, "running")
		doomed.Kill()

		// A second host is exactly what retries are for: the work has to land
		// somewhere, not die with the machine that happened to pick it up.
		stack.ConnectedAgent(admin, "retry-second", harness.WithQueues("build"))
		harness.WaitFor(t, 3*time.Minute, "the build to run again on the surviving host", func() bool {
			job, err := admin.GetJob(enqueued.JobID)
			return err == nil && job.Attempts > 1 && job.WorkerID != doomed.WorkerID
		})
	})

	t.Run("Success - retries run out and the build lands in the dead-letter queue", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()
		stack.ConnectedAgent(admin, "dlq-host", harness.WithQueues("build"))

		// A job type no host implements fails immediately, every time — the cheapest
		// way to watch the retry budget run out.
		enqueued, err := admin.Enqueue(harness.EnqueueRequest{
			Queue: "build", Type: "no-such-handler", MaxRetries: 2,
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		job := harness.WaitJobStatus(admin, enqueued.JobID, 2*time.Minute, "dead")
		if job.Attempts < 2 {
			t.Errorf("attempts = %d, want the configured 2 before giving up", job.Attempts)
		}

		// The dead-letter queue is where an operator finds what needs attention.
		dead := harness.Eventually(t, 30*time.Second, "the job to appear in the DLQ",
			func() ([]harness.Job, bool) {
				jobs, err := admin.DLQ("build")
				return jobs, err == nil && len(jobs) > 0
			})
		found := false
		for _, j := range dead {
			if j.ID == enqueued.JobID {
				found = true
			}
		}
		if !found {
			t.Errorf("job %s is not in the dead-letter queue", enqueued.JobID)
		}
	})

	t.Run("Success - retrying the dead-letter queue puts its work back on the queue", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()
		agent := stack.ConnectedAgent(admin, "dlq-retry-host", harness.WithQueues("build"))

		enqueued, err := admin.Enqueue(harness.EnqueueRequest{
			Queue: "build", Type: "no-such-handler", MaxRetries: 1,
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		harness.WaitJobStatus(admin, enqueued.JobID, 2*time.Minute, "dead")

		// With the host gone, the requeued job stays where retry-all put it. This job
		// can only ever fail, so with a host connected it would be dead again inside the
		// same second — and every "it ran again" signal the API exposes (status, attempt
		// count, finish time, which is second-granularity) would read the same as before.
		agent.Kill()
		stack.WaitAllDisconnected(admin)

		count, err := admin.RetryDLQ("build")
		if err != nil {
			t.Fatalf("retry dlq: %v", err)
		}
		if count != 1 {
			t.Errorf("retry-all reported %d, want 1", count)
		}

		job, err := admin.GetJob(enqueued.JobID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if job.Status != "pending" {
			t.Errorf("status after retry-all = %q, want pending", job.Status)
		}
		// A retried job starts over: keeping the spent budget would let it die on the
		// first stumble no matter how many retries it was configured for.
		if job.Attempts != 0 {
			t.Errorf("attempts after retry-all = %d, want 0", job.Attempts)
		}
		if job.Error != "" {
			t.Errorf("the retried job still carries its old error %q", job.Error)
		}

		// It is queued work again, not a dead-letter entry.
		if jobs, err := admin.DLQ("build"); err != nil {
			t.Fatalf("list dlq: %v", err)
		} else if len(jobs) != 0 {
			t.Errorf("the dead-letter queue still holds %d job(s) after retry-all", len(jobs))
		}
		queued, err := admin.QueueJobs("build")
		if err != nil {
			t.Fatalf("list queue jobs: %v", err)
		}
		found := false
		for _, j := range queued {
			if j.ID == enqueued.JobID {
				found = true
			}
		}
		if !found {
			t.Errorf("the retried job is not waiting on its queue")
		}
	})

	t.Run("Success - a build that outlives its timeout is failed", func(t *testing.T) {
		stack := newStack(t, harness.WithLeaseGrace(2*time.Second, 2*time.Second))
		admin := stack.AdminClient()
		stack.ConnectedAgent(admin, "timeout-host", harness.WithQueues("build"))

		// The container sleeps far longer than the budget it was given.
		enqueued, err := admin.Enqueue(harness.EnqueueRequest{
			Queue:      "build",
			Type:       "container",
			TimeoutSec: 2,
			MaxRetries: 1,
			Payload:    containerPayload(t, "bash:5", "echo starting; sleep 120"),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		// Whether the host gives up first or core reaps the lease, the build must not
		// hang around as running.
		harness.WaitJobStatus(admin, enqueued.JobID, 3*time.Minute, "dead")
	})
}

// TestMisbehavingHosts uses a client that breaks the rules a real agent follows, to
// prove core cannot be talked into corrupting a build's history.
func TestMisbehavingHosts(t *testing.T) {
	t.Run("Success - a result for a build the host never had is ignored", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		reg := stack.ClaimWorker(admin, "liar-host")
		raw, err := stack.DialRawWorker(reg.WorkerID)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if err := raw.Register(1, "build"); err != nil {
			t.Fatalf("register: %v", err)
		}

		// Queue a build and claim it passed without ever being given it. Believing
		// this would let any host mark anyone's deploy green.
		enqueued, err := admin.Enqueue(harness.EnqueueRequest{Queue: "other", Type: "noop"})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		if err := raw.ReportResult(enqueued.JobID, true, "", 5); err != nil {
			t.Fatalf("report result: %v", err)
		}

		harness.Never(t, 3*time.Second, "an unassigned build to be marked done", func() bool {
			job, err := admin.GetJob(enqueued.JobID)
			return err == nil && job.Status == "done"
		})
	})

	t.Run("Success - a result for an unknown build changes nothing", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		reg := stack.ClaimWorker(admin, "ghost-reporter")
		raw, err := stack.DialRawWorker(reg.WorkerID)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if err := raw.Register(1, "build"); err != nil {
			t.Fatalf("register: %v", err)
		}
		if err := raw.ReportResult("00000000-0000-0000-0000-000000000000", true, "", 1); err != nil {
			t.Fatalf("report result: %v", err)
		}

		// Core must stay up and keep serving: one confused host cannot take the
		// server with it.
		harness.WaitFor(t, 15*time.Second, "core to stay healthy", func() bool {
			_, err := admin.ListQueues()
			return err == nil
		})
		stack.WaitConnected(admin, reg.WorkerID)
	})

	t.Run("Success - a late result does not overwrite the retry that replaced it", func(t *testing.T) {
		stack := newStack(t, harness.WithLeaseGrace(time.Second, time.Second))
		admin := stack.AdminClient()

		reg := stack.ClaimWorker(admin, "slow-reporter")
		raw, err := stack.DialRawWorker(reg.WorkerID)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if err := raw.Register(1, "build"); err != nil {
			t.Fatalf("register: %v", err)
		}

		enqueued, err := admin.Enqueue(harness.EnqueueRequest{
			Queue: "build", Type: "noop", TimeoutSec: 1, MaxRetries: 3,
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		assignment := raw.WaitForAssignment(30 * time.Second)

		// Say nothing until core has given up on this attempt and requeued it.
		harness.WaitJobStatus(admin, enqueued.JobID, time.Minute, "pending", "dead")

		// Now report success for the attempt core already wrote off. Accepting it
		// would contradict the retry that is already in flight.
		if err := raw.ReportResult(assignment.JobId, true, "", 10); err != nil {
			t.Fatalf("report late result: %v", err)
		}
		harness.Never(t, 3*time.Second, "a late result to mark a requeued build done", func() bool {
			job, err := admin.GetJob(enqueued.JobID)
			return err == nil && job.Status == "done"
		})
	})
}

// TestCoreRestartKeepsWork proves the claim that core is stateless: everything that
// matters is in Postgres, so a restart is invisible to the work in flight.
func TestCoreRestartKeepsWork(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()

	queued, err := admin.Trigger(echoSlug, map[string]any{"environment": "staging"})
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}

	stack.Restart()
	restarted := stack.AdminClient()

	// The build was queued before the restart and no host has seen it yet. It must
	// still be there, and it must still run.
	builds, err := restarted.Builds(echoSlug)
	if err != nil {
		t.Fatalf("list builds after restart: %v", err)
	}
	found := false
	for _, b := range builds {
		if b.RunID == queued.RunID {
			found = true
		}
	}
	if !found {
		t.Fatalf("the build queued before the restart is gone")
	}

	stack.ConnectedAgent(restarted, "post-restart-host", harness.WithQueues("build"))
	harness.WaitBuildStatus(restarted, echoSlug, queued.RunID, 2*time.Minute, "passed")
}

// TestDrainingHost covers taking a host out of rotation without losing work.
func TestDrainingHost(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()

	reg := stack.ClaimWorker(admin, "draining-host")
	raw, err := stack.DialRawWorker(reg.WorkerID)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := raw.Register(1, "build"); err != nil {
		t.Fatalf("register: %v", err)
	}
	stack.WaitConnected(admin, reg.WorkerID)

	// Deleting the host is how the panel asks a live agent to stand down.
	if err := admin.DeleteWorker(reg.WorkerID); err != nil {
		t.Fatalf("delete worker: %v", err)
	}
	harness.WaitFor(t, 30*time.Second, "the host to be told to stand down", func() bool {
		return len(raw.Drains()) > 0
	})

	// Work queued now has nowhere to go, and must wait rather than vanish into a
	// host that is shutting down.
	enqueued, err := admin.Enqueue(harness.EnqueueRequest{Queue: "build", Type: "noop"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	harness.Never(t, 3*time.Second, "a draining host to be given new work", func() bool {
		for _, a := range raw.Assignments() {
			if a.JobId == enqueued.JobID {
				return true
			}
		}
		return false
	})

	// A fresh host picks it up.
	stack.ConnectedAgent(admin, "replacement-host", harness.WithQueues("build"))
	harness.WaitJobStatus(admin, enqueued.JobID, time.Minute, "done")
}
