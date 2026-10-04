//go:build e2e

package e2e

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lute/api/e2e/harness"
)

// TestCLI drives the real lute command against core and a real worker, with an account
// key from a login and a service key from the environment, as a laptop and CI would.
func TestCLI(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()
	stack.ConnectedAgent(admin, "cli-host", harness.WithQueues("build"))

	account, err := admin.CreateAPIKey("laptop")
	if err != nil {
		t.Fatalf("create account key: %v", err)
	}
	service, err := admin.CreateServiceKey("ci")
	if err != nil {
		t.Fatalf("create service key: %v", err)
	}
	if service.Scope != "service" {
		t.Fatalf("service key scope = %q", service.Scope)
	}

	laptop := stack.CLI()
	ci := stack.CLI().With("LUTE_URL="+stack.BaseURL(), "LUTE_API_KEY="+service.Token)

	t.Run("Success - an account key logs in and acts as its owner", func(t *testing.T) {
		res := laptop.Run(account.Token+"\n", "login", "--url", stack.BaseURL(), "--api-key-stdin", "--insecure-storage")
		if res.Code != 0 {
			t.Fatalf("login: %s", res)
		}
		res = laptop.Run("", "whoami", "--json")
		var who struct {
			URL  string `json:"url"`
			Key  struct{ Name, Scope string }
			User *struct{ Email string }
		}
		decodeCLI(t, res, &who)
		if who.Key.Scope != "account" || who.User == nil || who.User.Email != harness.AdminEmail {
			t.Errorf("whoami = %+v, want the admin's account key", who)
		}
		if strings.Contains(res.Stdout+res.Stderr, account.Token) {
			t.Error("whoami printed the key")
		}
	})

	t.Run("Success - jobs show describes the parameters to pass", func(t *testing.T) {
		var job struct {
			Slug       string
			Parameters []struct{ Name, Type string }
		}
		decodeCLI(t, laptop.Run("", "jobs", "show", echoSlug, "--json"), &job)
		if job.Slug != echoSlug || len(job.Parameters) == 0 || job.Parameters[0].Name != "environment" {
			t.Errorf("job = %+v", job)
		}
	})

	var shortID string
	t.Run("Success - run --wait --json passes with the values given", func(t *testing.T) {
		res := laptop.Run("", "run", echoSlug, "-p", "environment=prod", "-p", "regions=eu-central,us-east", "--wait", "--json")
		if res.Code != 0 {
			t.Fatalf("run: %s", res)
		}
		var run harness.Run
		decodeCLI(t, res, &run)
		if run.Status != "done" || run.Job != echoSlug {
			t.Errorf("run = %+v, want a done run of %s", run, echoSlug)
		}
		if run.Params["ENVIRONMENT"] != "prod" || run.Params["REGIONS"] != "eu-central,us-east" {
			t.Errorf("params = %v", run.Params)
		}
		shortID = run.ID[:8]
	})

	t.Run("Success - logs -f by the short id is NDJSON ending with the run", func(t *testing.T) {
		if shortID == "" {
			t.Skip("no run from the previous step")
		}
		res := laptop.Run("", "logs", "#"+shortID, "-f", "--json")
		if res.Code != 0 {
			t.Fatalf("logs -f: %s", res)
		}
		var lines []string
		var final *harness.Run
		for _, raw := range strings.Split(strings.TrimSpace(res.Stdout), "\n") {
			var rec struct {
				Line *string      `json:"line"`
				Run  *harness.Run `json:"run"`
			}
			if err := json.Unmarshal([]byte(raw), &rec); err != nil {
				t.Fatalf("not NDJSON: %q: %v", raw, err)
			}
			switch {
			case rec.Line != nil:
				lines = append(lines, *rec.Line)
			case rec.Run != nil:
				final = rec.Run
			}
		}
		if !linesContain(lines, "environment=prod") || !linesContain(lines, "build finished") {
			t.Errorf("the log misses the job's output:\n%s", strings.Join(lines, "\n"))
		}
		if final == nil || final.Status != "done" {
			t.Errorf("the stream does not end with the done run: %+v", final)
		}
	})

	t.Run("Fail - a failing job exits 1 under a service key", func(t *testing.T) {
		res := ci.Run("", "run", failingSlug, "--wait", "--json")
		if res.Code != 1 {
			t.Fatalf("exit = %d, want 1: %s", res.Code, res)
		}
		var run harness.Run
		decodeCLI(t, res, &run)
		if run.Status != "dead" && run.Status != "failed" {
			t.Errorf("status = %q, want dead or failed", run.Status)
		}
	})

	t.Run("Success - the account key sees the service key's runs", func(t *testing.T) {
		var list struct{ Runs []harness.Run }
		decodeCLI(t, laptop.Run("", "runs", "--job", failingSlug, "--json"), &list)
		if len(list.Runs) != 1 || list.Runs[0].Job != failingSlug {
			t.Errorf("runs of %s = %+v, want the service key's one run", failingSlug, list.Runs)
		}
	})

	t.Run("Fail - exit codes say what went wrong", func(t *testing.T) {
		if res := laptop.Run("", "run", "no-such-job", "--json"); res.Code != 4 {
			t.Errorf("unknown job: %s, want exit 4", res)
		}

		res := ci.Run("", "run", echoSlug, "-p", "retries=lots", "--json")
		if res.Code != 2 {
			t.Fatalf("bad value: %s, want exit 2", res)
		}
		var body struct {
			Error struct {
				Code   string
				Fields map[string]string
			}
		}
		if err := json.Unmarshal([]byte(res.Stderr), &body); err != nil || body.Error.Code != "validation_failed" || body.Error.Fields["retries"] == "" {
			t.Errorf("stderr = %q, want validation_failed naming retries", res.Stderr)
		}

		doomed, err := admin.CreateServiceKey("doomed")
		if err != nil {
			t.Fatal(err)
		}
		if err := admin.RevokeAPIKey(doomed.ID); err != nil {
			t.Fatal(err)
		}
		if res := stack.CLI().With("LUTE_URL="+stack.BaseURL(), "LUTE_API_KEY="+doomed.Token).Run("", "runs"); res.Code != 3 {
			t.Errorf("revoked key: %s, want exit 3", res)
		}
	})
}

func decodeCLI(t *testing.T, res harness.CLIResult, out any) {
	t.Helper()
	if res.Code != 0 {
		if err := json.Unmarshal([]byte(res.Stdout), out); err != nil {
			t.Fatalf("lute failed: %s", res)
		}
		return
	}
	if err := json.Unmarshal([]byte(res.Stdout), out); err != nil {
		t.Fatalf("decode lute output: %v\n%s", err, res)
	}
}
