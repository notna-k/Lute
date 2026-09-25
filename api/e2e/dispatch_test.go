//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/lute/api/e2e/harness"
)

// TestDispatchWaitsForACapableHost covers the decisions core makes about who gets a
// build: which queue, which labels, and how much a host can take at once.
func TestDispatchWaitsForACapableHost(t *testing.T) {
	t.Run("Success - work queued with no host waits, then runs when one arrives", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		build, err := admin.Trigger(echoSlug, map[string]any{"environment": "staging"})
		if err != nil {
			t.Fatalf("trigger: %v", err)
		}
		jobID := jobIDOf(t, admin, echoSlug, build.RunID)

		// Nothing can run it yet, and core must not pretend otherwise.
		harness.Never(t, 2*time.Second, "a build to start with no host connected", func() bool {
			job, err := admin.GetJob(jobID)
			return err == nil && job.Status != "pending"
		})

		// Connecting a host is the event that releases the work: no second trigger,
		// no waiting for the next sweep.
		stack.ConnectedAgent(admin, "late-host", harness.WithQueues("build"))
		harness.WaitBuildStatus(admin, echoSlug, build.RunID, 2*time.Minute, "passed")
	})

	t.Run("Success - a host only receives the queues it asked for", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()
		stack.ConnectedAgent(admin, "build-only", harness.WithQueues("build"))

		onDeploy, err := admin.Trigger(deploySlug, map[string]any{})
		if err != nil {
			t.Fatalf("trigger deploy build: %v", err)
		}
		deployJob := jobIDOf(t, admin, deploySlug, onDeploy.RunID)

		onBuild, err := admin.Trigger(echoSlug, map[string]any{"environment": "staging"})
		if err != nil {
			t.Fatalf("trigger build: %v", err)
		}

		// The build queue's work runs; the deploy queue's waits for a host that
		// serves it. Handing it over anyway would run deploys on the wrong fleet.
		harness.WaitBuildStatus(admin, echoSlug, onBuild.RunID, 2*time.Minute, "passed")
		job, err := admin.GetJob(deployJob)
		if err != nil {
			t.Fatalf("get deploy job: %v", err)
		}
		if job.Status != "pending" {
			t.Errorf("deploy job status = %q, want pending: it went to a host that does not serve its queue", job.Status)
		}
	})

	t.Run("Success - a label selector holds work until a matching host appears", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()
		agent := stack.ConnectedAgent(admin, "unlabelled-host", harness.WithQueues("build"))

		build, err := admin.Trigger(regionSlug, map[string]any{})
		if err != nil {
			t.Fatalf("trigger: %v", err)
		}
		jobID := jobIDOf(t, admin, regionSlug, build.RunID)

		harness.Never(t, 2*time.Second, "a selector build to run on a host without the label", func() bool {
			job, err := admin.GetJob(jobID)
			return err == nil && job.Status != "pending"
		})

		// Labelling the host is an operator action that must take effect at once:
		// waiting for a reconnect would make the panel's label editor a lie.
		if _, err := admin.PatchLabels(agent.WorkerID, map[string]string{"region": "eu"}); err != nil {
			t.Fatalf("patch labels: %v", err)
		}
		harness.WaitBuildStatus(admin, regionSlug, build.RunID, 2*time.Minute, "passed")
	})

	t.Run("Success - a host with capacity one runs its builds one at a time", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()
		stack.ConnectedAgent(admin, "serial-host", harness.WithQueues("build"), harness.WithConcurrency(1))

		first, err := admin.Trigger(slowSlug, map[string]any{})
		if err != nil {
			t.Fatalf("trigger first: %v", err)
		}
		second, err := admin.Trigger(echoSlug, map[string]any{"environment": "staging"})
		if err != nil {
			t.Fatalf("trigger second: %v", err)
		}

		firstJob := jobIDOf(t, admin, slowSlug, first.RunID)
		secondJob := jobIDOf(t, admin, echoSlug, second.RunID)

		harness.WaitJobStatus(admin, firstJob, time.Minute, "running")

		// Overcommitting a host is how a build machine falls over.
		harness.Never(t, 3*time.Second, "a second build to start on a host already at capacity", func() bool {
			job, err := admin.GetJob(secondJob)
			return err == nil && job.Status == "running"
		})

		connected := stack.WaitConnected(admin, stack.Agents()[0].WorkerID)
		if connected.ActiveJobs != 1 {
			t.Errorf("active jobs = %d, want 1", connected.ActiveJobs)
		}
	})

	t.Run("Success - a host with capacity two runs two builds at once", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()
		agent := stack.ConnectedAgent(admin, "parallel-host", harness.WithQueues("build"), harness.WithConcurrency(2))

		var jobs []string
		for range 2 {
			build, err := admin.Trigger(slowSlug, map[string]any{})
			if err != nil {
				t.Fatalf("trigger: %v", err)
			}
			jobs = append(jobs, jobIDOf(t, admin, slowSlug, build.RunID))
		}

		// Capacity the operator granted has to actually be used, or the fleet is
		// idle while the queue grows.
		harness.WaitFor(t, time.Minute, "both builds to be running at once", func() bool {
			running := 0
			for _, id := range jobs {
				if job, err := admin.GetJob(id); err == nil && job.Status == "running" {
					running++
				}
			}
			return running == 2
		})

		connected := stack.WaitConnected(admin, agent.WorkerID)
		if connected.ActiveJobs != 2 {
			t.Errorf("active jobs = %d, want 2", connected.ActiveJobs)
		}
	})

	t.Run("Success - higher priority work is dispatched first", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		// Queue everything before any host exists, so the order is core's choice
		// rather than an accident of arrival time.
		low, err := admin.Enqueue(harness.EnqueueRequest{Queue: "build", Type: "noop", Priority: 1})
		if err != nil {
			t.Fatalf("enqueue low: %v", err)
		}
		high, err := admin.Enqueue(harness.EnqueueRequest{Queue: "build", Type: "noop", Priority: 100})
		if err != nil {
			t.Fatalf("enqueue high: %v", err)
		}

		reg := stack.ClaimWorker(admin, "priority-host")
		raw, err := stack.DialRawWorker(reg.WorkerID)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if err := raw.Register(1, "build"); err != nil {
			t.Fatalf("register: %v", err)
		}

		first := raw.WaitForAssignment(30 * time.Second)
		if first.JobId != high.JobID {
			t.Errorf("first assignment = %s, want the high-priority job %s", first.JobId, high.JobID)
		}
		// The host has capacity for one, so the cheaper job must still be waiting:
		// priority that only holds when the queue is idle is no priority at all.
		lowJob, err := admin.GetJob(low.JobID)
		if err != nil {
			t.Fatalf("get the low-priority job: %v", err)
		}
		if lowJob.Status != "pending" {
			t.Errorf("low-priority job status = %q, want pending", lowJob.Status)
		}
	})

	t.Run("Success - a delayed job waits out its delay and then runs", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()
		stack.ConnectedAgent(admin, "delay-host", harness.WithQueues("build"))

		enqueued, err := admin.Enqueue(harness.EnqueueRequest{
			Queue: "build", Type: "noop", DelayMs: 3000,
		})
		if err != nil {
			t.Fatalf("enqueue delayed: %v", err)
		}

		// Running it early defeats every use of a delay: backoff, batching, a
		// scheduled window.
		harness.Never(t, 1500*time.Millisecond, "a delayed job to run before its time", func() bool {
			job, err := admin.GetJob(enqueued.JobID)
			return err == nil && job.Status != "pending"
		})
		harness.WaitJobStatus(admin, enqueued.JobID, 30*time.Second, "done")
	})

	t.Run("Success - a cancelled job is never handed to a host", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		enqueued, err := admin.Enqueue(harness.EnqueueRequest{Queue: "build", Type: "noop"})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		if err := admin.CancelJob(enqueued.JobID); err != nil {
			t.Fatalf("cancel: %v", err)
		}

		reg := stack.ClaimWorker(admin, "cancel-host")
		raw, err := stack.DialRawWorker(reg.WorkerID)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if err := raw.Register(1, "build"); err != nil {
			t.Fatalf("register: %v", err)
		}

		harness.Never(t, 3*time.Second, "a cancelled job to be dispatched", func() bool {
			return len(raw.Assignments()) > 0
		})
		job, err := admin.GetJob(enqueued.JobID)
		if err != nil {
			t.Fatalf("get job: %v", err)
		}
		if job.Status != "dead" {
			t.Errorf("cancelled job status = %q, want dead", job.Status)
		}
	})

	t.Run("Success - purging a queue drops what is waiting on it", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		for range 3 {
			if _, err := admin.Enqueue(harness.EnqueueRequest{Queue: "build", Type: "noop"}); err != nil {
				t.Fatalf("enqueue: %v", err)
			}
		}

		deleted, err := admin.PurgeQueue("build")
		if err != nil {
			t.Fatalf("purge: %v", err)
		}
		if deleted != 3 {
			t.Errorf("purge reported %d deleted, want 3", deleted)
		}
		remaining, err := admin.QueueJobs("build")
		if err != nil {
			t.Fatalf("list queue jobs: %v", err)
		}
		if len(remaining) != 0 {
			t.Errorf("%d job(s) survived the purge", len(remaining))
		}
	})

	t.Run("Success - two hosts share the work", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()
		first := stack.ConnectedAgent(admin, "pair-one", harness.WithQueues("build"), harness.WithConcurrency(1))
		second := stack.ConnectedAgent(admin, "pair-two", harness.WithQueues("build"), harness.WithConcurrency(1))

		for range 2 {
			if _, err := admin.Trigger(slowSlug, map[string]any{}); err != nil {
				t.Fatalf("trigger: %v", err)
			}
		}

		// Both hosts busy is the point of having two. Stacking both builds on one
		// while the other idles is a scheduler that does not scale.
		harness.WaitFor(t, time.Minute, "both hosts to be busy", func() bool {
			workers, err := admin.ConnectedWorkers()
			if err != nil {
				return false
			}
			busy := map[string]bool{}
			for _, w := range workers {
				if w.ActiveJobs > 0 {
					busy[w.WorkerID] = true
				}
			}
			return busy[first.WorkerID] && busy[second.WorkerID]
		})
	})
}
