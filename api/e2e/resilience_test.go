//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/lute/api/e2e/harness"
)

// TestLostHostsAndRetries: a build must never sit in "running" because its host went away.
func TestLostHostsAndRetries(t *testing.T) {
	t.Run("Success - a killed host's build is failed, not left running", func(t *testing.T) {
		stack := newStack(t, harness.WithLeaseGrace(2*time.Second, 2*time.Second))
		admin := stack.AdminClient()
		agent := stack.ConnectedAgent(admin, "doomed-host", harness.WithQueues("build"))

		// Enqueued directly for a short timeout: a lost build is measured against timeout
		// plus lease grace, and definitions cannot set a timeout (they all get 300s).
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

		agent.Kill()

		// Core has to notice on its own, or the build reads as running forever.
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

		// Still running when its host dies, yet short enough for the lease to expire in-test.
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

		// The retry must land on the second host, not die with the first.
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

		// A job type no host implements fails at once, which exhausts retries cheaply.
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

		// With a host connected this always-failing job would be dead again within the
		// second, indistinguishable from before; with none it stays where retry-all put it.
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
		// A retried job gets its whole retry budget back.
		if job.Attempts != 0 {
			t.Errorf("attempts after retry-all = %d, want 0", job.Attempts)
		}
		if job.Error != "" {
			t.Errorf("the retried job still carries its old error %q", job.Error)
		}

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

		// Whether the host times out or core reaps the lease, it must not stay running.
		harness.WaitJobStatus(admin, enqueued.JobID, 3*time.Minute, "dead")
	})
}

// TestMisbehavingHosts proves a rule-breaking client cannot corrupt a build's history.
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

		// Claim success for a build never assigned: believing it would let any host mark any deploy green.
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

		harness.WaitJobStatus(admin, enqueued.JobID, time.Minute, "pending", "dead")

		// Success for an attempt core already wrote off would contradict the retry in flight.
		if err := raw.ReportResult(assignment.JobId, true, "", 10); err != nil {
			t.Fatalf("report late result: %v", err)
		}
		harness.Never(t, 3*time.Second, "a late result to mark a requeued build done", func() bool {
			job, err := admin.GetJob(enqueued.JobID)
			return err == nil && job.Status == "done"
		})
	})
}

// TestCoreRestartKeepsWork proves core is stateless: a restart is invisible to work in flight.
func TestCoreRestartKeepsWork(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()

	queued, err := admin.Trigger(echoSlug, map[string]any{"environment": "staging"})
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}

	stack.Restart()
	restarted := stack.AdminClient()

	// Queued before the restart and not yet dispatched: it must still exist and run.
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

	if err := admin.DeleteWorker(reg.WorkerID); err != nil {
		t.Fatalf("delete worker: %v", err)
	}
	harness.WaitFor(t, 30*time.Second, "the host to be told to stand down", func() bool {
		return len(raw.Drains()) > 0
	})

	// Work queued now must wait, not vanish into a host that is shutting down.
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

	stack.ConnectedAgent(admin, "replacement-host", harness.WithQueues("build"))
	harness.WaitJobStatus(admin, enqueued.JobID, time.Minute, "done")
}
