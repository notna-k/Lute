//go:build e2e

package e2e

import (
	"archive/zip"
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lute/api/e2e/harness"
)

// TestDefinitionsComeFromGit covers Git as the source of truth, panel edits as visible drift,
// and neither silently overwriting the other.
func TestDefinitionsComeFromGit(t *testing.T) {
	t.Run("Success - definitions in the directory are synced at startup", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		defs, err := admin.ListJobDefs()
		if err != nil {
			t.Fatalf("list definitions: %v", err)
		}
		for _, slug := range []string{echoSlug, failingSlug, regionSlug, deploySlug} {
			def := findDef(t, defs, slug)
			if def.GitState != "synced" {
				t.Errorf("%s gitState = %q, want synced", slug, def.GitState)
			}
		}

		// The panel renders the run form from the schema, so it must survive YAML intact.
		echo := findDef(t, defs, echoSlug)
		if echo.Queue != "build" {
			t.Errorf("queue = %q, want build", echo.Queue)
		}
		if echo.Runtime != "bash:5" {
			t.Errorf("runtime = %q, want bash:5", echo.Runtime)
		}
		if len(echo.Parameters) != 5 {
			t.Errorf("got %d parameters, want the 5 the file declares", len(echo.Parameters))
		}
		region := findDef(t, defs, regionSlug)
		if got := region.LabelSelector["region"]; got != "eu" {
			t.Errorf("label selector region = %q, want eu", got)
		}
	})

	t.Run("Success - a panel edit shows as drift and can be reverted", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		before, err := admin.GetJobDef(echoSlug)
		if err != nil {
			t.Fatalf("get definition: %v", err)
		}

		edited, err := admin.UpdateJobDef(echoSlug, harness.JobDefinitionRequest{
			Name:       before.Name,
			Queue:      before.Queue,
			Runtime:    "bash:5",
			Command:    `echo "edited in the panel"`,
			Parameters: before.Parameters,
		})
		if err != nil {
			t.Fatalf("update definition: %v", err)
		}
		if edited.GitState != "modified" {
			t.Errorf("gitState after an edit = %q, want modified: the panel would not flag the drift", edited.GitState)
		}

		// The export is how the edit gets committed, so it must contain the edit.
		yaml, err := admin.ExportJobDef(echoSlug)
		if err != nil {
			t.Fatalf("export: %v", err)
		}
		if !strings.Contains(yaml, "edited in the panel") {
			t.Errorf("the exported YAML does not carry the edit:\n%s", yaml)
		}

		reverted, err := admin.RevertJobDef(echoSlug)
		if err != nil {
			t.Fatalf("revert: %v", err)
		}
		if reverted.GitState != "synced" {
			t.Errorf("gitState after revert = %q, want synced", reverted.GitState)
		}
		if reverted.Command == edited.Command {
			t.Error("revert left the panel's command in place")
		}
	})

	t.Run("Success - a sync picks up a new file and leaves unrelated edits alone", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		// An edit to one definition must not be undone by a commit that touches another.
		before, err := admin.GetJobDef(failingSlug)
		if err != nil {
			t.Fatalf("get definition: %v", err)
		}
		if _, err := admin.UpdateJobDef(failingSlug, harness.JobDefinitionRequest{
			Name:    before.Name,
			Queue:   before.Queue,
			Runtime: before.Runtime,
			Command: `echo "panel edit that must survive"`,
		}); err != nil {
			t.Fatalf("update definition: %v", err)
		}

		stack.WriteJobDef("added-later.yaml", `
name: Added Later
queue: build
runtime: bash:5
command: echo added
`)
		res, err := admin.SyncJobDefs()
		if err != nil {
			t.Fatalf("sync: %v", err)
		}
		if res.Added != 1 {
			t.Errorf("sync added %d, want 1", res.Added)
		}

		defs, err := admin.ListJobDefs()
		if err != nil {
			t.Fatalf("list definitions: %v", err)
		}
		findDef(t, defs, "added-later")

		survived := findDef(t, defs, failingSlug)
		if !strings.Contains(survived.Command, "panel edit that must survive") {
			t.Errorf("the sync overwrote an edit to a file it did not touch: command = %q", survived.Command)
		}
	})

	t.Run("Success - a definition whose file is gone is kept and flagged", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		stack.RemoveJobDef("deploy-only.yaml")
		if _, err := admin.SyncJobDefs(); err != nil {
			t.Fatalf("sync: %v", err)
		}

		defs, err := admin.ListJobDefs()
		if err != nil {
			t.Fatalf("list definitions: %v", err)
		}
		// Deleting a file must not silently destroy a job's history.
		orphan := findDef(t, defs, deploySlug)
		if orphan.GitState != "removed" {
			t.Errorf("gitState of a definition with no file = %q, want removed", orphan.GitState)
		}
	})

	t.Run("Success - with pruning on, a deleted file deletes the definition", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		on := true
		if _, err := admin.UpdateSettings(harness.SettingsUpdate{PruneDefinitions: &on}); err != nil {
			t.Fatalf("enable pruning: %v", err)
		}

		stack.RemoveJobDef("deploy-only.yaml")
		res, err := admin.SyncJobDefs()
		if err != nil {
			t.Fatalf("sync: %v", err)
		}
		if res.Pruned != 1 {
			t.Errorf("sync pruned %d, want 1", res.Pruned)
		}
		if _, err := admin.GetJobDef(deploySlug); harness.StatusOf(err) != http.StatusNotFound {
			t.Errorf("get a pruned definition: err = %v, want 404", err)
		}
	})

	t.Run("Success - a panel-authored definition is flagged as not in Git", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		created, err := admin.CreateJobDef(harness.JobDefinitionRequest{
			Name:    "Hand Written",
			Queue:   "build",
			Runtime: "bash:5",
			Command: "echo hand written",
		})
		if err != nil {
			t.Fatalf("create definition: %v", err)
		}
		if created.Slug != "hand-written" {
			t.Errorf("slug = %q, want hand-written", created.Slug)
		}
		if created.GitState == "synced" {
			t.Error("a definition with no file in Git reports itself as synced")
		}

		// Creating the same name twice must not quietly replace the first.
		if _, err := admin.CreateJobDef(harness.JobDefinitionRequest{
			Name: "Hand Written", Runtime: "bash:5", Command: "echo again",
		}); harness.StatusOf(err) != http.StatusConflict {
			t.Errorf("create a duplicate definition: err = %v, want 409", err)
		}
	})

	t.Run("Fail - a definition missing what it needs to run is refused", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		cases := []struct {
			name  string
			field string
			req   harness.JobDefinitionRequest
		}{
			{"no name", "name", harness.JobDefinitionRequest{Runtime: "bash:5", Command: "echo"}},
			{"no runtime", "runtime", harness.JobDefinitionRequest{Name: "No Runtime", Command: "echo"}},
			{"no command", "command", harness.JobDefinitionRequest{Name: "No Command", Runtime: "bash:5"}},
			{"unknown parameter type", "parameters", harness.JobDefinitionRequest{
				Name: "Bad Param", Runtime: "bash:5", Command: "echo",
				Parameters: []harness.ParameterField{{Name: "x", Type: "quantum"}},
			}},
		}
		for _, tc := range cases {
			_, err := admin.CreateJobDef(tc.req)
			if harness.StatusOf(err) != http.StatusBadRequest {
				t.Errorf("create with %s: err = %v, want 400", tc.name, err)
				continue
			}
			if _, ok := harness.FieldsOf(err)[tc.field]; !ok {
				t.Errorf("create with %s: no detail for %q in %v", tc.name, tc.field, harness.FieldsOf(err))
			}
		}
	})

	t.Run("Success - the export archive holds one document per definition", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		raw, err := admin.ExportJobDefsZip()
		if err != nil {
			t.Fatalf("export zip: %v", err)
		}
		r, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			t.Fatalf("read zip: %v", err)
		}
		defs, err := admin.ListJobDefs()
		if err != nil {
			t.Fatalf("list definitions: %v", err)
		}
		if len(r.File) != len(defs) {
			t.Errorf("the archive holds %d files for %d definitions", len(r.File), len(defs))
		}
		for _, f := range r.File {
			if !strings.HasSuffix(f.Name, ".yaml") && !strings.HasSuffix(f.Name, ".yml") {
				t.Errorf("archive entry %q is not YAML", f.Name)
			}
		}
	})
}

// TestAdhocBuildPolicy covers the setting that allows builds with a schema not in Git.
func TestAdhocBuildPolicy(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()
	stack.ConnectedAgent(admin, "adhoc-host", harness.WithQueues("build"))

	// A schema the panel edited: one extra input the committed definition does not have.
	edited := []harness.ParameterField{
		{Name: "environment", Type: "select", EnvVar: "ENVIRONMENT", Required: true, Default: "staging",
			Options: []harness.Option{{Value: "staging"}, {Value: "prod"}}},
		{Name: "extra", Type: "string", EnvVar: "EXTRA", Default: "added-in-panel"},
	}

	t.Run("Fail - with ad-hoc builds off, a schema that differs from Git is refused", func(t *testing.T) {
		off := false
		if _, err := admin.UpdateSettings(harness.SettingsUpdate{AllowAdhocBuilds: &off}); err != nil {
			t.Fatalf("disable ad-hoc builds: %v", err)
		}

		_, err := admin.TriggerWithSchema(echoSlug, map[string]any{"environment": "staging"}, edited)
		if harness.StatusOf(err) != http.StatusConflict {
			t.Fatalf("trigger an ad-hoc build with the policy off: err = %v, want 409", err)
		}
		if code := harness.CodeOf(err); code != "conflict" {
			t.Errorf("error code = %q, want conflict", code)
		}
	})

	t.Run("Success - with ad-hoc builds on, the submitted schema is what runs", func(t *testing.T) {
		on := true
		if _, err := admin.UpdateSettings(harness.SettingsUpdate{AllowAdhocBuilds: &on}); err != nil {
			t.Fatalf("enable ad-hoc builds: %v", err)
		}

		build, err := admin.TriggerWithSchema(echoSlug, map[string]any{
			"environment": "prod",
			"extra":       "from-the-panel",
		}, edited)
		if err != nil {
			t.Fatalf("trigger ad-hoc build: %v", err)
		}
		if !build.AdHoc {
			t.Error("the build does not record that it ran a panel-edited schema")
		}
		// The added input must be passed through, not silently dropped.
		if got := build.Params["EXTRA"]; got != "from-the-panel" {
			t.Errorf("EXTRA = %q, want from-the-panel", got)
		}
		harness.WaitBuildStatus(admin, echoSlug, build.RunID, 2*time.Minute, "passed")
	})

	t.Run("Success - running the committed definition is never ad-hoc", func(t *testing.T) {
		off := false
		if _, err := admin.UpdateSettings(harness.SettingsUpdate{AllowAdhocBuilds: &off}); err != nil {
			t.Fatalf("disable ad-hoc builds: %v", err)
		}

		// With the policy off, committed definitions must still build.
		build, err := admin.Trigger(echoSlug, map[string]any{"environment": "staging"})
		if err != nil {
			t.Fatalf("trigger the committed definition with ad-hoc off: %v", err)
		}
		if build.AdHoc {
			t.Error("a build of the committed definition was marked ad-hoc")
		}
	})
}
