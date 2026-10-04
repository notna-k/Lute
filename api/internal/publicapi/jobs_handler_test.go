package publicapi

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/types"
	"github.com/lute/api/internal/runs"
)

var runIDPattern = regexp.MustCompile(`^[0-9a-f]{24}$`)

// webRelease has one input of each kind a client has to handle.
func webRelease() *models.JobDefinition {
	return &models.JobDefinition{
		Slug: "web-release",
		JobSpec: models.JobSpec{
			Name:          "Web release",
			Description:   "Build and ship the web app",
			Queue:         "deploy",
			LabelSelector: map[string]string{"region": "eu"},
			Runtime:       "node:25-alpine",
			Command:       "./scripts/ship.sh",
			SourceRepo:    "https://git.example.com/web.git",
			Parameters: []models.ParameterField{
				{
					Name: "environment", Type: "select", Label: "Environment", Required: true,
					Options: []models.ParameterOption{
						{Value: "staging", Label: "Staging"},
						{Value: "prod", Label: "Production", Hint: "Customers see it"},
					},
				},
				{
					Name: "regions", Type: "multiselect", Label: "Regions",
					Options: []models.ParameterOption{{Value: "eu-central"}, {Value: "us-east"}},
				},
				{Name: "dry_run", Type: "bool", Label: "Dry run", Default: false},
				{Name: "release_date", Type: "date", Label: "Release date", EnvVar: "SHIP_ON"},
				{Name: "token", Type: "secret", Label: "Deploy token", SecretRef: "deploy-token", Default: "should-not-leak"},
			},
		},
	}
}

// enqueue records run at a fixed age, so "newest" never hangs on two
// inserts landing in one millisecond.
func (f *fixture) enqueue(run *models.Run, age time.Duration) *models.Run {
	f.t.Helper()
	if run.Queue == "" {
		run.Queue = "build"
	}
	run.Type = "container"
	run.CreatedAt = types.NewMilliTime(time.Now().Add(-age))
	got, _, err := f.runs.Enqueue(context.Background(), run, runs.Job{})
	if err != nil {
		f.t.Fatalf("enqueue: %v", err)
	}
	return got
}

func TestListJobs(t *testing.T) {
	f := newFixture(t)
	anton, bob := f.user("anton@acme.dev"), f.user("bob@acme.dev")
	antonKey := f.key(anton, "laptop", "account")
	serviceKey := f.key(anton, "ci", "service")
	f.job(webRelease())
	f.job(&models.JobDefinition{Slug: "lint"})

	list := func(token string) []JobSummary {
		t.Helper()
		rec := f.public(token, http.MethodGet, "/jobs", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
		}
		return decode[struct {
			Jobs []JobSummary `json:"jobs"`
		}](t, rec).Jobs
	}

	t.Run("Success - every job is listed by slug, without runs before any ran", func(t *testing.T) {
		jobs := list(antonKey.Token)
		if len(jobs) != 2 || jobs[0].Slug != "lint" || jobs[1].Slug != "web-release" {
			t.Fatalf("jobs = %+v, want lint and web-release in slug order", jobs)
		}
		web := jobs[1]
		if web.Name != "Web release" || web.Queue != "deploy" || web.Runtime != "node:25-alpine" || web.Description == "" {
			t.Errorf("web-release = %+v", web)
		}
		for _, j := range jobs {
			if j.LastRun != nil {
				t.Errorf("%s has a last run before any ran: %+v", j.Slug, j.LastRun)
			}
		}
	})

	t.Run("Success - the list stays lean: schemas are on the job itself", func(t *testing.T) {
		rec := f.public(antonKey.Token, http.MethodGet, "/jobs", nil)
		if strings.Contains(rec.Body.String(), `"parameters"`) {
			t.Errorf("the list carries parameter schemas: %s", rec.Body)
		}
	})

	t.Run("Success - last_run is the newest run the key can see", func(t *testing.T) {
		f.enqueue(&models.Run{UserID: anton.ID, JobSlug: "lint"}, 3*time.Minute)
		antonsNewest := f.enqueue(&models.Run{UserID: anton.ID, JobSlug: "lint"}, 2*time.Minute)
		bobsNewest := f.enqueue(&models.Run{UserID: bob.ID, JobSlug: "lint"}, time.Minute)

		lint := list(antonKey.Token)[0]
		if lint.LastRun == nil || lint.LastRun.ID != antonsNewest.ID.Hex() {
			t.Errorf("anton's last lint run = %+v, want %s (bob's run is not anton's)", lint.LastRun, antonsNewest.ID)
		}
		if lint.LastRun != nil && lint.LastRun.Job != "lint" {
			t.Errorf("last_run.job = %q, want lint", lint.LastRun.Job)
		}

		lint = list(serviceKey.Token)[0]
		if lint.LastRun == nil || lint.LastRun.ID != bobsNewest.ID.Hex() {
			t.Errorf("the service key's last lint run = %+v, want bob's %s: it sees every run", lint.LastRun, bobsNewest.ID)
		}
	})

	t.Run("Fail - no key is refused", func(t *testing.T) {
		wantError(t, f.public("", http.MethodGet, "/jobs", nil), http.StatusUnauthorized, "unauthorized")
	})
}

func TestGetJob(t *testing.T) {
	f := newFixture(t)
	anton := f.user("anton@acme.dev")
	key := f.key(anton, "laptop", "account")
	f.job(webRelease())
	f.job(&models.JobDefinition{Slug: "lint", JobSpec: models.JobSpec{Command: "make lint"}})

	get := func(slug string) (JobDetail, string) {
		t.Helper()
		rec := f.public(key.Token, http.MethodGet, "/jobs/"+slug, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
		}
		return decode[JobDetail](t, rec), rec.Body.String()
	}
	param := func(d JobDetail, name string) ParameterResponse {
		t.Helper()
		i := slices.IndexFunc(d.Parameters, func(p ParameterResponse) bool { return p.Name == name })
		if i < 0 {
			t.Fatalf("parameter %q is missing from %+v", name, d.Parameters)
		}
		return d.Parameters[i]
	}

	t.Run("Success - the job says what it runs and where", func(t *testing.T) {
		d, _ := get("web-release")
		if d.Slug != "web-release" || d.Name != "Web release" || d.Queue != "deploy" || d.Runtime != "node:25-alpine" {
			t.Errorf("summary = %+v", d.JobSummary)
		}
		if d.Command != "./scripts/ship.sh" || d.SourceRepo != "https://git.example.com/web.git" {
			t.Errorf("command = %q, source_repo = %q", d.Command, d.SourceRepo)
		}
	})

	t.Run("Success - every parameter keeps its order, type and env var", func(t *testing.T) {
		d, _ := get("web-release")
		var names []string
		for _, p := range d.Parameters {
			names = append(names, p.Name)
		}
		if want := []string{"environment", "regions", "dry_run", "release_date", "token"}; !slices.Equal(names, want) {
			t.Fatalf("parameters = %v, want %v", names, want)
		}

		env := param(d, "environment")
		if env.Type != "select" || !env.Required || env.Label != "Environment" || env.EnvVar != "ENVIRONMENT" {
			t.Errorf("environment = %+v", env)
		}
		wantOpts := []OptionResponse{{Value: "staging", Label: "Staging"}, {Value: "prod", Label: "Production", Hint: "Customers see it"}}
		if !slices.Equal(env.Options, wantOpts) {
			t.Errorf("environment options = %+v, want %+v", env.Options, wantOpts)
		}
		if got := param(d, "release_date").EnvVar; got != "SHIP_ON" {
			t.Errorf("release_date env_var = %q, want the explicit SHIP_ON", got)
		}
		if got := param(d, "regions"); got.Type != "multiselect" || got.EnvVar != "REGIONS" || got.Required {
			t.Errorf("regions = %+v", got)
		}
	})

	t.Run("Success - a false default is still a default", func(t *testing.T) {
		_, raw := get("web-release")
		var body struct {
			Parameters []map[string]any `json:"parameters"`
		}
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			t.Fatal(err)
		}
		for _, p := range body.Parameters {
			if p["name"] != "dry_run" {
				continue
			}
			if v, ok := p["default"]; !ok || v != false {
				t.Errorf("dry_run default = %v (present %v), want false", v, ok)
			}
		}
	})

	t.Run("Success - a secret is listed, but nothing about its value", func(t *testing.T) {
		d, raw := get("web-release")
		token := param(d, "token")
		if token.Type != "secret" || token.Default != nil {
			t.Errorf("token = %+v, want type secret and no default", token)
		}
		for _, leak := range []string{"should-not-leak", "deploy-token"} {
			if strings.Contains(raw, leak) {
				t.Errorf("the response contains %q: %s", leak, raw)
			}
		}
	})

	t.Run("Success - a job without inputs answers an empty list, not null", func(t *testing.T) {
		d, raw := get("lint")
		if len(d.Parameters) != 0 || !strings.Contains(raw, `"parameters":[]`) {
			t.Errorf("parameters in %s, want []", raw)
		}
	})

	t.Run("Success - the newest visible run comes along", func(t *testing.T) {
		run := f.enqueue(&models.Run{UserID: anton.ID, JobSlug: "lint"}, 0)
		d, _ := get("lint")
		if d.LastRun == nil || d.LastRun.ID != run.ID.Hex() {
			t.Errorf("last_run = %+v, want %s", d.LastRun, run.ID)
		}
	})

	t.Run("Fail - an unknown job is not found", func(t *testing.T) {
		wantError(t, f.public(key.Token, http.MethodGet, "/jobs/nope", nil), http.StatusNotFound, "not_found")
	})
}

func TestStartJobRun(t *testing.T) {
	f := newFixture(t)
	anton, bob := f.user("anton@acme.dev"), f.user("bob@acme.dev")
	antonKey := f.key(anton, "laptop", "account")
	bobKey := f.key(bob, "laptop", "account")
	serviceKey := f.key(anton, "ci", "service")
	f.job(webRelease())
	f.job(&models.JobDefinition{Slug: "lint", JobSpec: models.JobSpec{Command: "make lint"}})

	start := func(token, slug string, body any, header http.Header) (RunResponse, int) {
		t.Helper()
		h := http.Header{"Authorization": {"Bearer " + token}}
		for k, v := range header {
			h[k] = v
		}
		rec := f.do(http.MethodPost, publicBase+"/jobs/"+slug+"/runs", body, h)
		if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
			t.Fatalf("start %s: %d %s", slug, rec.Code, rec.Body)
		}
		return decode[RunResponse](t, rec), rec.Code
	}
	staging := map[string]any{"params": map[string]any{
		"environment":  "staging",
		"regions":      []string{"eu-central", "us-east"},
		"dry_run":      true,
		"release_date": "2026-10-05",
	}}

	t.Run("Success - valid values start a run of the definition", func(t *testing.T) {
		run, status := start(antonKey.Token, "web-release", staging, nil)
		if status != http.StatusCreated {
			t.Errorf("status = %d, want 201", status)
		}
		if !runIDPattern.MatchString(run.ID) {
			t.Errorf("id = %q, want 24 hex characters", run.ID)
		}
		if run.Job != "web-release" || run.Queue != "deploy" || run.Type != "container" || run.Status != "pending" {
			t.Errorf("run = %+v", run)
		}
		want := map[string]string{"ENVIRONMENT": "staging", "REGIONS": "eu-central,us-east", "DRY_RUN": "true", "SHIP_ON": "2026-10-05"}
		if len(run.Params) != len(want) {
			t.Errorf("params = %v, want %v", run.Params, want)
		}
		for k, v := range want {
			if run.Params[k] != v {
				t.Errorf("params[%s] = %q, want %q", k, run.Params[k], v)
			}
		}
	})

	t.Run("Success - the queued job carries the definition", func(t *testing.T) {
		before := len(f.gw.dispatched)
		run, _ := start(antonKey.Token, "web-release", staging, nil)
		stored, err := f.runs.Visible(context.Background(), runs.UserViewer(anton.ID), run.ID)
		if err != nil {
			t.Fatalf("the run was not stored: %v", err)
		}
		job, err := f.queue.GetJob(context.Background(), stored.JobID)
		if err != nil {
			t.Fatalf("the run is not on the queue: %v", err)
		}
		var spec map[string]any
		if err := json.Unmarshal(job.Payload, &spec); err != nil {
			t.Fatalf("payload %s: %v", job.Payload, err)
		}
		if spec["runtime"] != "node:25-alpine" || spec["command"] != "./scripts/ship.sh" || spec["source_repository"] != "https://git.example.com/web.git" {
			t.Errorf("payload = %v", spec)
		}
		if job.Selector["region"] != "eu" {
			t.Errorf("selector = %v, want region=eu from the definition", job.Selector)
		}
		if len(f.gw.dispatched) != before+1 || f.gw.dispatched[before] != "deploy" {
			t.Errorf("dispatched = %v, want deploy once more", f.gw.dispatched)
		}
	})

	t.Run("Success - left-out values take their defaults", func(t *testing.T) {
		run, _ := start(antonKey.Token, "web-release", map[string]any{"params": map[string]any{"environment": "prod"}}, nil)
		if run.Params["DRY_RUN"] != "false" {
			t.Errorf("DRY_RUN = %q, want the default false", run.Params["DRY_RUN"])
		}
		if _, ok := run.Params["REGIONS"]; ok {
			t.Errorf("REGIONS = %q, want it unset: it has no default", run.Params["REGIONS"])
		}
	})

	t.Run("Success - a bool may arrive as a string, as the CLI sends it", func(t *testing.T) {
		run, _ := start(antonKey.Token, "web-release", map[string]any{"params": map[string]any{"environment": "prod", "dry_run": "true"}}, nil)
		if run.Params["DRY_RUN"] != "true" {
			t.Errorf("DRY_RUN = %q, want true", run.Params["DRY_RUN"])
		}
	})

	t.Run("Success - a repeated Idempotency-Key returns the first run", func(t *testing.T) {
		header := http.Header{"Idempotency-Key": {"release-2026-10-05"}}
		first, status := start(antonKey.Token, "web-release", staging, header)
		if status != http.StatusCreated {
			t.Fatalf("first status = %d, want 201", status)
		}
		again, status := start(antonKey.Token, "web-release", staging, header)
		if status != http.StatusOK || again.ID != first.ID {
			t.Errorf("repeat = %d %s, want 200 with the first run %s", status, again.ID, first.ID)
		}

		// Keys are per owner: bob's request with the same key is his own run.
		bobs, status := start(bobKey.Token, "web-release", staging, header)
		if status != http.StatusCreated || bobs.ID == first.ID {
			t.Errorf("bob's run = %d %s, want a new run", status, bobs.ID)
		}
	})

	t.Run("Success - an empty body runs a job without required inputs", func(t *testing.T) {
		run, status := start(antonKey.Token, "lint", nil, nil)
		if status != http.StatusCreated || run.Job != "lint" {
			t.Errorf("run = %d %+v", status, run)
		}
	})

	t.Run("Success - a service key's run is seen by the key and by users", func(t *testing.T) {
		run, _ := start(serviceKey.Token, "lint", map[string]any{"params": map[string]any{}}, nil)
		for name, token := range map[string]string{"service key": serviceKey.Token, "anton": antonKey.Token, "bob": bobKey.Token} {
			if rec := f.public(token, http.MethodGet, "/runs/"+run.ID, nil); rec.Code != http.StatusOK {
				t.Errorf("%s reads the service key's run: %d %s", name, rec.Code, rec.Body)
			}
		}
	})

	t.Run("Fail - missing and bad values name their parameters, and nothing is queued", func(t *testing.T) {
		before := len(f.gw.dispatched)
		rec := f.public(antonKey.Token, http.MethodPost, "/jobs/web-release/runs", map[string]any{"params": map[string]any{
			"regions":      []string{"mars"},
			"dry_run":      "maybe",
			"release_date": "next tuesday",
		}})
		e := wantError(t, rec, http.StatusBadRequest, "validation_failed")
		for field, want := range map[string]string{
			"environment":  "required",
			"regions":      "not an allowed option",
			"dry_run":      "true or false",
			"release_date": "YYYY-MM-DD",
		} {
			if !strings.Contains(e.Error.Fields[field], want) {
				t.Errorf("fields[%s] = %q, want it to mention %q", field, e.Error.Fields[field], want)
			}
		}
		if len(f.gw.dispatched) != before {
			t.Errorf("a rejected run was dispatched: %v", f.gw.dispatched[before:])
		}
	})

	t.Run("Fail - an unknown job is not found", func(t *testing.T) {
		rec := f.public(antonKey.Token, http.MethodPost, "/jobs/nope/runs", map[string]any{})
		wantError(t, rec, http.StatusNotFound, "not_found")
	})

	t.Run("Fail - a body that is not JSON is a bad request", func(t *testing.T) {
		rec := f.public(antonKey.Token, http.MethodPost, "/jobs/lint/runs", `{"params":`)
		wantError(t, rec, http.StatusBadRequest, "bad_request")
	})

	t.Run("Fail - no key is refused", func(t *testing.T) {
		wantError(t, f.public("", http.MethodPost, "/jobs/lint/runs", nil), http.StatusUnauthorized, "unauthorized")
	})
}
