//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/lute/api/e2e/harness"
)

// TestBuildRuns is the path Lute exists for: a definition from Git, a build triggered
// from the panel, a container on a build host, and the result and logs coming back.
func TestBuildRuns(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()
	agent := stack.ConnectedAgent(admin, "build-host", harness.WithQueues("build"), harness.WithConcurrency(2))

	t.Run("Success - a build runs, passes, and reports how long it took", func(t *testing.T) {
		build, err := admin.Trigger(echoSlug, map[string]any{
			"environment": "prod",
			"regions":     []string{"eu-central", "us-east"},
			"dry_run":     false,
			"retries":     5,
		})
		if err != nil {
			t.Fatalf("trigger: %v", err)
		}
		// Trigger dispatches before it answers, so with an idle host connected the
		// build can already be running by the time the response is written. What must
		// not happen is a brand-new build reporting an outcome.
		if build.Status != "queued" && build.Status != "running" {
			t.Errorf("a freshly triggered build reports %q, want queued or running", build.Status)
		}

		done := harness.WaitBuildStatus(admin, echoSlug, build.RunID, 2*time.Minute, "passed", "failed")
		if done.Status != "passed" {
			t.Fatalf("build finished %q, want passed\nagent:\n%s", done.Status, agent.Stderr())
		}
		if done.DurationMs <= 0 {
			t.Errorf("duration = %d ms, want the measured runtime", done.DurationMs)
		}

		// The parameters the operator chose are what the container must have seen.
		lines := readAllLogs(t, admin, done.JobID)
		for _, want := range []string{
			"environment=prod",
			"regions=eu-central,us-east",
			"dry_run=false",
			"retries=5",
			"build finished",
		} {
			if !linesContain(lines, want) {
				t.Errorf("no log line contains %q; the parameter never reached the container\nlog:\n%s",
					want, strings.Join(lines, "\n"))
			}
		}

		// And core must have recorded the attempt against the host that ran it.
		exec := harness.WaitExecution(admin, done.JobID, 30*time.Second)
		if exec.WorkerID != agent.WorkerID {
			t.Errorf("execution worker = %q, want %q", exec.WorkerID, agent.WorkerID)
		}
		if !exec.Success {
			t.Errorf("execution records a failure for a passing build: %q", exec.Error)
		}
		if exec.Queue != "build" {
			t.Errorf("execution queue = %q, want build", exec.Queue)
		}
	})

	t.Run("Success - defaults fill in the parameters the operator left alone", func(t *testing.T) {
		build, err := admin.Trigger(echoSlug, map[string]any{})
		if err != nil {
			t.Fatalf("trigger with no values: %v", err)
		}
		done := harness.WaitBuildStatus(admin, echoSlug, build.RunID, 2*time.Minute, "passed", "failed")
		if done.Status != "passed" {
			t.Fatalf("build finished %q, want passed", done.Status)
		}

		lines := readAllLogs(t, admin, done.JobID)
		for _, want := range []string{"environment=staging", "regions=eu-central", "dry_run=true", "retries=2"} {
			if !linesContain(lines, want) {
				t.Errorf("no log line contains %q; the schema default was not applied\nlog:\n%s",
					want, strings.Join(lines, "\n"))
			}
		}
	})

	t.Run("Success - a secret parameter is never echoed back to the panel", func(t *testing.T) {
		build, err := admin.Trigger(echoSlug, map[string]any{
			"environment":  "staging",
			"deploy_token": "super-secret-value",
		})
		if err != nil {
			t.Fatalf("trigger: %v", err)
		}

		// A build's params are shown in the panel and offered as the next build's
		// starting point. A secret landing there leaks it to every viewer.
		for key, value := range build.Params {
			if strings.Contains(value, "super-secret-value") {
				t.Errorf("parameter %q carries the submitted secret", key)
			}
		}
		if _, ok := build.Params["DEPLOY_TOKEN"]; ok {
			t.Error("the secret parameter was resolved into the build's params")
		}
	})

	t.Run("Fail - a build whose command exits non-zero is reported failed", func(t *testing.T) {
		build, err := admin.Trigger(failingSlug, map[string]any{})
		if err != nil {
			t.Fatalf("trigger: %v", err)
		}

		done := harness.WaitBuildStatus(admin, failingSlug, build.RunID, 3*time.Minute, "failed")
		if done.Status != "failed" {
			t.Fatalf("build finished %q, want failed", done.Status)
		}

		// The output that explains the failure has to survive it.
		lines := readAllLogs(t, admin, done.JobID)
		if !linesContain(lines, "about to fail") {
			t.Errorf("the failing build's output was lost\nlog:\n%s", strings.Join(lines, "\n"))
		}

		exec := harness.WaitExecution(admin, done.JobID, 30*time.Second)
		if exec.Success {
			t.Error("execution records a success for a failing build")
		}
		if exec.Error == "" {
			t.Error("execution records no reason for the failure")
		}
	})

	t.Run("Fail - values the schema rejects do not start a build", func(t *testing.T) {
		before, err := admin.Builds(echoSlug)
		if err != nil {
			t.Fatalf("list builds: %v", err)
		}

		_, err = admin.Trigger(echoSlug, map[string]any{
			"environment": "not-an-option",
			"retries":     "not-a-number",
		})
		if harness.StatusOf(err) != http.StatusBadRequest {
			t.Fatalf("trigger with invalid values: err = %v, want 400", err)
		}
		if code := harness.CodeOf(err); code != "invalid_parameters" {
			t.Errorf("error code = %q, want invalid_parameters", code)
		}
		// The panel renders these next to the offending inputs; without them the
		// operator only learns that something, somewhere, was wrong.
		fields := harness.FieldsOf(err)
		for _, name := range []string{"environment", "retries"} {
			if _, ok := fields[name]; !ok {
				t.Errorf("no per-field detail for %q; got %v", name, fields)
			}
		}

		after, err := admin.Builds(echoSlug)
		if err != nil {
			t.Fatalf("list builds: %v", err)
		}
		if len(after) != len(before) {
			t.Errorf("a rejected trigger queued a build anyway (%d -> %d)", len(before), len(after))
		}
	})

	t.Run("Fail - triggering a definition that does not exist is a 404", func(t *testing.T) {
		if _, err := admin.Trigger("no-such-job", map[string]any{}); harness.StatusOf(err) != http.StatusNotFound {
			t.Fatalf("trigger an unknown definition: err = %v, want 404", err)
		}
	})
}

// TestBuildLogs covers reading a build's output, which core fetches from the host that
// ran it rather than storing itself.
func TestBuildLogs(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()
	stack.ConnectedAgent(admin, "log-host", harness.WithQueues("build"))

	build, err := admin.Trigger(echoSlug, map[string]any{"environment": "staging"})
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	done := harness.WaitBuildStatus(admin, echoSlug, build.RunID, 2*time.Minute, "passed", "failed")
	if done.Status != "passed" {
		t.Fatalf("build finished %q, want passed", done.Status)
	}
	jobID := done.JobID

	t.Run("Success - the tail of the log is what the panel opens on", func(t *testing.T) {
		page, err := admin.JobLogs(jobID, harness.LogOptions{Direction: "tail", Limit: 200})
		if err != nil {
			t.Fatalf("read tail: %v", err)
		}
		if len(page.Lines) == 0 {
			t.Fatal("no log lines returned for a build that printed output")
		}
		if page.Direction != "tail" {
			t.Errorf("direction = %q, want tail", page.Direction)
		}
		if page.FileSize <= 0 {
			t.Errorf("file_size = %d, want the log's size", page.FileSize)
		}
		if !linesContain(page.Lines, "build finished") {
			t.Errorf("the tail is missing the last thing the build printed:\n%s", strings.Join(page.Lines, "\n"))
		}
	})

	t.Run("Success - the head of the log starts at the beginning", func(t *testing.T) {
		page, err := admin.JobLogs(jobID, harness.LogOptions{Direction: "head", Limit: 200})
		if err != nil {
			t.Fatalf("read head: %v", err)
		}
		if len(page.Lines) == 0 {
			t.Fatal("no log lines returned reading from the start")
		}
	})

	t.Run("Success - paging back through the log reaches earlier lines", func(t *testing.T) {
		first, err := admin.JobLogs(jobID, harness.LogOptions{Direction: "tail", Limit: 2})
		if err != nil {
			t.Fatalf("read tail: %v", err)
		}
		if !first.HasMore {
			t.Skip("the build's log fits in two lines; nothing to page")
		}
		if first.NextCursor == "" {
			t.Fatal("has_more is set but no cursor was returned; the panel cannot scroll back")
		}

		older, err := admin.JobLogs(jobID, harness.LogOptions{
			Direction: "tail", Limit: 2, Cursor: first.NextCursor,
		})
		if err != nil {
			t.Fatalf("read older page: %v", err)
		}
		if len(older.Lines) == 0 {
			t.Fatal("paging with the returned cursor produced nothing")
		}
		// Two pages that return the same lines mean an infinite scroll that never moves.
		if diff := cmp.Diff(first.Lines, older.Lines); diff == "" {
			t.Errorf("the older page repeated the newest lines:\n%s", strings.Join(older.Lines, "\n"))
		}
	})

	t.Run("Fail - a bad direction or limit is refused", func(t *testing.T) {
		if _, err := admin.JobLogs(jobID, harness.LogOptions{Direction: "sideways"}); harness.StatusOf(err) != http.StatusBadRequest {
			t.Errorf("direction=sideways: err = %v, want 400", err)
		}
		if _, err := admin.JobLogs(jobID, harness.LogOptions{Limit: -1}); harness.StatusOf(err) != http.StatusBadRequest {
			t.Errorf("limit=-1: err = %v, want 400", err)
		}
	})

	t.Run("Fail - logs of a build whose host is gone answer rather than hang", func(t *testing.T) {
		// Core keeps no copy of the log, so when the host disappears it can only say so.
		for _, a := range stack.Agents() {
			a.Kill()
		}
		stack.WaitAllDisconnected(admin)

		_, err := admin.JobLogs(jobID, harness.LogOptions{Limit: 10})
		if status := harness.StatusOf(err); status != http.StatusServiceUnavailable {
			t.Fatalf("read logs with no host connected: err = %v, want 503", err)
		}
	})

	t.Run("Fail - logs for an unknown build are a 404", func(t *testing.T) {
		if _, err := admin.JobLogs("00000000-0000-0000-0000-000000000000", harness.LogOptions{}); harness.StatusOf(err) != http.StatusNotFound {
			t.Fatalf("logs of an unknown build: err = %v, want 404", err)
		}
	})
}

// TestDocumentedRuntimes runs the kind of image Lute's own documentation advertises.
func TestDocumentedRuntimes(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()
	agent := stack.ConnectedAgent(admin, "runtime-host", harness.WithQueues("build"))

	t.Run("Success - an alpine runtime runs its command", func(t *testing.T) {
		build, err := admin.Trigger(alpineSlug, map[string]any{})
		if err != nil {
			t.Fatalf("trigger: %v", err)
		}

		done := harness.WaitBuildStatus(admin, alpineSlug, build.RunID, 3*time.Minute, "passed", "failed")
		if done.Status != "passed" {
			lines := readAllLogs(t, admin, done.JobID)
			t.Fatalf("a build on alpine:3 finished %q, want passed.\nlog:\n%s\nagent:\n%s",
				done.Status, strings.Join(lines, "\n"), agent.Stderr())
		}
		if lines := readAllLogs(t, admin, done.JobID); !linesContain(lines, "alpine ran") {
			t.Errorf("the alpine build produced no output\nlog:\n%s", strings.Join(lines, "\n"))
		}
	})
}

// readAllLogs returns a build's whole log, newest page first being irrelevant here:
// the assertions are about content, not order of retrieval.
func readAllLogs(t *testing.T, c *harness.Client, jobID string) []string {
	t.Helper()
	page, err := c.JobLogs(jobID, harness.LogOptions{Direction: "head", Limit: 500})
	if err != nil {
		t.Fatalf("read logs of %s: %v", jobID, err)
	}
	return page.Lines
}
