//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/lute/api/e2e/harness"
)

// TestDeleteDrainsTheWorker: deleting a busy worker lets its build finish, gives it no new
// work, then forgets it while the agent stops for good.
func TestDeleteDrainsTheWorker(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()
	agent := stack.ConnectedAgent(admin, "drain-on-delete", harness.WithQueues("build"), harness.WithConcurrency(2))

	running, err := admin.Enqueue(harness.EnqueueRequest{
		Queue:      "build",
		Type:       "container",
		TimeoutSec: 120,
		Payload:    containerPayload(t, "bash:5", "echo begin; sleep 6; echo end"),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	harness.WaitJobStatus(admin, running.JobID, time.Minute, "running")

	result, err := admin.DeleteWorker(agent.WorkerID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if result != "deleting" {
		t.Fatalf("delete result = %q, want deleting while a job runs", result)
	}
	if w, err := admin.GetWorker(agent.WorkerID); err != nil || w.Status != "deleting" {
		t.Errorf("worker status = %q (%v), want deleting", w.Status, err)
	}

	queued, err := admin.Enqueue(harness.EnqueueRequest{Queue: "build", Type: "noop"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	harness.Never(t, 2*time.Second, "a deleted worker to be given new work", func() bool {
		job, err := admin.GetJob(queued.JobID)
		return err == nil && job.WorkerID == agent.WorkerID
	})

	// The running build is allowed to finish.
	harness.WaitJobStatus(admin, running.JobID, time.Minute, "done")
	agent.WaitExit(time.Minute)
	if agent.ExitCode() != 0 {
		t.Errorf("exit code = %d, want 0:\n%s", agent.ExitCode(), agent.Stderr())
	}
	if _, err := admin.GetWorker(agent.WorkerID); harness.StatusOf(err) != 404 {
		t.Errorf("get deleted worker: err = %v, want 404", err)
	}
	if job, _ := admin.GetJob(queued.JobID); job.Status != "pending" {
		t.Errorf("the job queued during the drain is %q, want pending for another worker", job.Status)
	}
}

// TestStopDrainsTheAgent: `docker stop` (SIGTERM) must never kill a running build.
func TestStopDrainsTheAgent(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()
	agent := stack.ConnectedAgent(admin, "stopped-host", harness.WithQueues("build"), harness.WithConcurrency(2))

	running, err := admin.Enqueue(harness.EnqueueRequest{
		Queue:      "build",
		Type:       "container",
		TimeoutSec: 120,
		Payload:    containerPayload(t, "bash:5", "echo begin; sleep 6; echo end"),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	harness.WaitJobStatus(admin, running.JobID, time.Minute, "running")

	agent.Terminate()
	harness.WaitFor(t, 15*time.Second, "core to see the agent draining", func() bool {
		workers, err := admin.ConnectedWorkers()
		if err != nil {
			return false
		}
		for _, w := range workers {
			if w.WorkerID == agent.WorkerID {
				return w.Draining
			}
		}
		return false
	})

	queued, err := admin.Enqueue(harness.EnqueueRequest{Queue: "build", Type: "noop"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	harness.Never(t, 2*time.Second, "a draining agent to be given new work", func() bool {
		job, err := admin.GetJob(queued.JobID)
		return err == nil && job.WorkerID == agent.WorkerID
	})

	harness.WaitJobStatus(admin, running.JobID, time.Minute, "done")
	agent.WaitExit(time.Minute)
	if agent.ExitCode() != 0 {
		t.Errorf("exit code = %d, want 0:\n%s", agent.ExitCode(), agent.Stderr())
	}
	// Stopping is not deleting: the worker comes back with the same identity.
	agent.Restart()
	stack.WaitConnected(admin, agent.WorkerID)
	harness.WaitJobStatus(admin, queued.JobID, time.Minute, "done")
}

// TestKilledAgentLeavesNothingBehind: a job container outlives a kill -9 of its agent, and
// the agent's next start removes it with its volume and network.
func TestKilledAgentLeavesNothingBehind(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()
	agent := stack.ConnectedAgent(admin, "reaped-host", harness.WithQueues("build"))

	enqueued, err := admin.Enqueue(harness.EnqueueRequest{
		Queue:      "build",
		Type:       "container",
		TimeoutSec: 300,
		Payload:    containerPayload(t, "bash:5", "echo started; sleep 300"),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	harness.WaitJobStatus(admin, enqueued.JobID, time.Minute, "running")
	harness.WaitFor(t, time.Minute, "the job's container, volume and network", func() bool {
		return len(harness.JobResources(enqueued.JobID)) >= 3
	})

	agent.Kill()
	if left := harness.JobResources(enqueued.JobID); len(left) < 3 {
		t.Fatalf("the sibling job container should survive its agent; found %v", left)
	}

	agent.Restart()
	stack.WaitConnected(admin, agent.WorkerID)
	harness.WaitFor(t, 30*time.Second, "the restarted agent to reap the leftovers", func() bool {
		return len(harness.JobResources(enqueued.JobID)) == 0
	})
}

// TestRepositoryJob clones in a container of its own and records what ran in meta.json.
func TestRepositoryJob(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()
	agent := stack.ConnectedAgent(admin, "repo-host", harness.WithQueues("build"))

	payload, err := json.Marshal(harness.ContainerSpec{
		SourceRepository: "https://github.com/octocat/Hello-World",
		Runtime:          "alpine:3",
		Command:          "pwd; cat README",
	})
	if err != nil {
		t.Fatal(err)
	}
	enqueued, err := admin.Enqueue(harness.EnqueueRequest{Queue: "build", Type: "container", TimeoutSec: 120, Payload: payload})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	job := harness.WaitJobStatus(admin, enqueued.JobID, 2*time.Minute, "done", "dead")
	if job.Status != "done" {
		t.Fatalf("job ended %s: %s", job.Status, job.Error)
	}

	page, err := admin.JobLogs(enqueued.JobID, harness.LogOptions{Limit: 200})
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	for _, want := range []string{"Hello World!", "/workspace", "clone ok"} {
		if !linesContain(page.Lines, want) {
			t.Errorf("the log has no %q:\n%v", want, page.Lines)
		}
	}

	raw, err := os.ReadFile(filepath.Join(agent.DataDir, "jobs", enqueued.JobID, "meta.json"))
	if err != nil {
		t.Fatalf("meta.json: %v", err)
	}
	var meta struct {
		Repository  string `json:"repository"`
		Commit      string `json:"commit"`
		Image       string `json:"image"`
		ImageDigest string `json:"image_digest"`
		ExitCode    *int   `json:"exit_code"`
		FinishedAt  string `json:"finished_at"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("decode meta.json: %v\n%s", err, raw)
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(meta.Commit) {
		t.Errorf("commit = %q, want a sha", meta.Commit)
	}
	if meta.Image != "alpine:3" || meta.ImageDigest == "" || meta.FinishedAt == "" {
		t.Errorf("meta.json = %s", raw)
	}
	if meta.ExitCode == nil || *meta.ExitCode != 0 {
		t.Errorf("exit code = %v, want 0", meta.ExitCode)
	}
	if left := harness.JobResources(enqueued.JobID); len(left) != 0 {
		t.Errorf("the finished job left %v behind", left)
	}
}
