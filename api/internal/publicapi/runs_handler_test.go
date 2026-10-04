package publicapi

import (
	"context"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
)

type runList struct {
	Runs  []RunResponse `json:"runs"`
	Total int64         `json:"total"`
}

func (f *fixture) listRuns(token, query string) runList {
	f.t.Helper()
	rec := f.public(token, http.MethodGet, "/runs"+query, nil)
	if rec.Code != http.StatusOK {
		f.t.Fatalf("list runs%s: %d %s", query, rec.Code, rec.Body)
	}
	return decode[runList](f.t, rec)
}

func ids(runs []RunResponse) []string {
	out := make([]string, 0, len(runs))
	for _, r := range runs {
		out = append(out, r.ID)
	}
	return out
}

func TestRunIDs(t *testing.T) {
	f := newFixture(t)
	anton := f.user("anton@acme.dev")
	key := f.key(anton, "laptop", "account")
	run := f.enqueue(&models.Run{UserID: anton.ID, JobSlug: "lint"}, 0)
	full := run.ID.Hex()

	t.Run("Success - a run is addressed by its full id or its short form", func(t *testing.T) {
		for _, ref := range []string{full, full[:8], full[:12]} {
			rec := f.public(key.Token, http.MethodGet, "/runs/"+ref, nil)
			if rec.Code != http.StatusOK {
				t.Errorf("GET /runs/%s: %d %s", ref, rec.Code, rec.Body)
				continue
			}
			if got := decode[RunResponse](t, rec); got.ID != full {
				t.Errorf("GET /runs/%s returned %s", ref, got.ID)
			}
		}
	})

	t.Run("Success - the short form works for logs, cancel and retry too", func(t *testing.T) {
		if rec := f.public(key.Token, http.MethodGet, "/runs/"+full[:8]+"/logs", nil); rec.Code == http.StatusNotFound && decode[apiError](t, rec).Error.Message == "run not found" {
			t.Errorf("logs by short id: the run was not found: %s", rec.Body)
		}
		rec := f.public(key.Token, http.MethodDelete, "/runs/"+full[:8], nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("cancel by short id: %d %s", rec.Code, rec.Body)
		}
		if got := decode[RunResponse](t, rec); got.Status != "dead" {
			t.Errorf("cancelled run status = %q, want dead", got.Status)
		}
		if rec := f.public(key.Token, http.MethodPost, "/runs/"+full[:8]+"/retry", nil); rec.Code != http.StatusOK {
			t.Errorf("retry by short id: %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("Success - upper case is the same id", func(t *testing.T) {
		upper := []byte(full[:8])
		for i, c := range upper {
			if c >= 'a' && c <= 'f' {
				upper[i] = c - 'a' + 'A'
			}
		}
		if rec := f.public(key.Token, http.MethodGet, "/runs/"+string(upper), nil); rec.Code != http.StatusOK {
			t.Errorf("GET /runs/%s: %d", upper, rec.Code)
		}
	})

	t.Run("Fail - a prefix two runs share is ambiguous", func(t *testing.T) {
		shared := "abcdef01"
		for range 2 {
			r := &models.Run{UserID: anton.ID}
			r.ID = id.ID(shared + id.New().Hex()[8:])
			f.enqueue(r, 0)
		}
		e := wantError(t, f.public(key.Token, http.MethodGet, "/runs/"+shared, nil), http.StatusBadRequest, "bad_request")
		if e.Error.Message == "" {
			t.Error("the ambiguity is not explained")
		}
	})

	t.Run("Fail - too short, too long or not hex is not found", func(t *testing.T) {
		for _, ref := range []string{full[:7], full + "0", "zzzzzzzz", "3f6c1d2e-8a4b-4c1f-9e7d-5b2a0c9d8e71", "%25%25%25%25%25%25%25%25"} {
			wantError(t, f.public(key.Token, http.MethodGet, "/runs/"+ref, nil), http.StatusNotFound, "not_found")
		}
	})
}

func TestRunVisibility(t *testing.T) {
	f := newFixture(t)
	anton, bob := f.user("anton@acme.dev"), f.user("bob@acme.dev")
	antonKey := f.key(anton, "laptop", "account")
	bobKey := f.key(bob, "laptop", "account")
	serviceKey := f.key(anton, "ci", "service")

	antons := f.enqueue(&models.Run{UserID: anton.ID}, 3*time.Minute).ID.Hex()
	bobs := f.enqueue(&models.Run{UserID: bob.ID}, 2*time.Minute).ID.Hex()
	createdByService := f.public(serviceKey.Token, http.MethodPost, "/runs", map[string]any{"queue": "build", "type": "noop"})
	if createdByService.Code != http.StatusCreated {
		t.Fatalf("a service key creates a run: %d %s", createdByService.Code, createdByService.Body)
	}
	services := decode[RunResponse](t, createdByService).ID

	t.Run("Success - an account key sees its user's runs and service-key runs", func(t *testing.T) {
		got := ids(f.listRuns(antonKey.Token, "").Runs)
		if !slices.Equal(got, []string{services, antons}) {
			t.Errorf("anton sees %v, want [%s %s] newest first", got, services, antons)
		}
	})

	t.Run("Success - a service key sees every run", func(t *testing.T) {
		got := f.listRuns(serviceKey.Token, "")
		if !slices.Equal(ids(got.Runs), []string{services, bobs, antons}) || got.Total != 3 {
			t.Errorf("the service key sees %v (total %d), want all three", ids(got.Runs), got.Total)
		}
		if rec := f.public(serviceKey.Token, http.MethodGet, "/runs/"+bobs[:8], nil); rec.Code != http.StatusOK {
			t.Errorf("the service key reads bob's run: %d", rec.Code)
		}
	})

	t.Run("Success - a service key's idempotency key is its own", func(t *testing.T) {
		body := map[string]any{"queue": "build", "type": "noop", "idempotency_key": "nightly"}
		first := f.public(serviceKey.Token, http.MethodPost, "/runs", body)
		again := f.public(serviceKey.Token, http.MethodPost, "/runs", body)
		if first.Code != http.StatusCreated || again.Code != http.StatusOK ||
			decode[RunResponse](t, first).ID != decode[RunResponse](t, again).ID {
			t.Errorf("repeat = %d then %d, want 201 then 200 with the same run", first.Code, again.Code)
		}
		other := f.key(anton, "other-ci", "service")
		if rec := f.public(other.Token, http.MethodPost, "/runs", body); rec.Code != http.StatusCreated {
			t.Errorf("another service key's first use of the key: %d, want 201", rec.Code)
		}
	})

	t.Run("Fail - another user's run reads as missing", func(t *testing.T) {
		wantError(t, f.public(antonKey.Token, http.MethodGet, "/runs/"+bobs, nil), http.StatusNotFound, "not_found")
		wantError(t, f.public(antonKey.Token, http.MethodGet, "/runs/"+bobs[:8], nil), http.StatusNotFound, "not_found")
		wantError(t, f.public(bobKey.Token, http.MethodDelete, "/runs/"+antons, nil), http.StatusNotFound, "not_found")
	})
}

func TestRunFilters(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	anton := f.user("anton@acme.dev")
	key := f.key(anton, "laptop", "account")

	// Each on its own queue, so a dequeue takes exactly that run.
	pending := f.enqueue(&models.Run{UserID: anton.ID, Queue: "q-pending", JobSlug: "deploy"}, 4*time.Minute)
	running := f.enqueue(&models.Run{UserID: anton.ID, Queue: "q-running", JobSlug: "deploy"}, 3*time.Minute)
	done := f.enqueue(&models.Run{UserID: anton.ID, Queue: "q-done", JobSlug: "lint"}, 2*time.Minute)
	failed := f.enqueue(&models.Run{UserID: anton.ID, Queue: "q-failed", JobSlug: "lint"}, time.Minute)

	dequeue := func(q string) {
		t.Helper()
		if job, err := f.queue.Dequeue(ctx, q, "worker-1"); err != nil || job == nil {
			t.Fatalf("dequeue %s: %v %v", q, job, err)
		}
	}
	dequeue("q-running")
	dequeue("q-done")
	if err := f.queue.Complete(ctx, done.JobID, 1200); err != nil {
		t.Fatal(err)
	}
	if err := f.execs.Upsert(ctx, &models.JobExecution{JobID: done.JobID, Success: true, ElapsedMs: 1200}); err != nil {
		t.Fatal(err)
	}
	// A failed execution whose queue slot is gone: the record alone says failed.
	if err := f.execs.Upsert(ctx, &models.JobExecution{JobID: failed.JobID, Success: false}); err != nil {
		t.Fatal(err)
	}
	if err := f.queue.DeleteJob(ctx, failed.JobID); err != nil {
		t.Fatal(err)
	}

	t.Run("Success - status filters agree with the status each run reports", func(t *testing.T) {
		for status, want := range map[string]*models.Run{"pending": pending, "running": running, "done": done, "failed": failed} {
			got := f.listRuns(key.Token, "?status="+status)
			if len(got.Runs) != 1 || got.Runs[0].ID != want.ID.Hex() || got.Total != 1 {
				t.Errorf("status=%s lists %v, want only %s", status, ids(got.Runs), want.ID)
				continue
			}
			if got.Runs[0].Status != status {
				t.Errorf("the run listed under %s says it is %s", status, got.Runs[0].Status)
			}
		}
		if got := f.listRuns(key.Token, "?status=dead"); len(got.Runs) != 0 {
			t.Errorf("status=dead lists %v, want none", ids(got.Runs))
		}
	})

	t.Run("Success - a job filter lists only that job's runs, carrying the job", func(t *testing.T) {
		got := f.listRuns(key.Token, "?job=lint")
		if !slices.Equal(ids(got.Runs), []string{failed.ID.Hex(), done.ID.Hex()}) {
			t.Errorf("job=lint lists %v", ids(got.Runs))
		}
		for _, r := range got.Runs {
			if r.Job != "lint" {
				t.Errorf("run %s says job %q, want lint", r.ID, r.Job)
			}
		}
	})

	t.Run("Success - filters combine and page", func(t *testing.T) {
		if got := f.listRuns(key.Token, "?job=deploy&status=running"); !slices.Equal(ids(got.Runs), []string{running.ID.Hex()}) {
			t.Errorf("job=deploy&status=running lists %v", ids(got.Runs))
		}
		page := f.listRuns(key.Token, "?limit=1&offset=1")
		if len(page.Runs) != 1 || page.Runs[0].ID != done.ID.Hex() || page.Total != 4 {
			t.Errorf("second page of one = %v (total %d), want the done run of 4", ids(page.Runs), page.Total)
		}
	})

	t.Run("Fail - an unknown status names the field", func(t *testing.T) {
		e := wantError(t, f.public(key.Token, http.MethodGet, "/runs?status=passed", nil), http.StatusBadRequest, "validation_failed")
		if e.Error.Fields["status"] == "" {
			t.Errorf("fields = %v, want status", e.Error.Fields)
		}
	})
}
