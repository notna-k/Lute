package jobdefs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lute/api/internal/db/models"
)

func spec(command string) models.JobSpec {
	return models.JobSpec{Name: "build", Queue: "default", Runtime: "alpine", Command: command}
}

// synced is a definition as a previous sync left it: live spec equal to Git's.
func synced(slug, command string) models.JobDefinition {
	s := spec(command)
	return models.JobDefinition{Slug: slug, JobSpec: s, SourcePath: slug + ".yaml", GitSpec: &s}
}

func fromGit(slug, command string) *models.JobDefinition {
	return &models.JobDefinition{Slug: slug, JobSpec: spec(command), SourcePath: slug + ".yaml"}
}

func TestReconcileCreatesNewDefinitions(t *testing.T) {
	plan := reconcile(nil, []*models.JobDefinition{fromGit("a", "make")}, false)

	if len(plan.creates) != 1 || plan.result.Added != 1 {
		t.Fatalf("want 1 create, got %+v", plan)
	}
	if got := plan.creates[0].GitState(); got != models.GitSynced {
		t.Errorf("new definition state = %q, want synced", got)
	}
}

func TestReconcileKeepsPanelEditWhenGitUnchanged(t *testing.T) {
	edited := synced("a", "make")
	edited.Command = "make -j8" // edited in the panel

	plan := reconcile([]models.JobDefinition{edited}, []*models.JobDefinition{fromGit("a", "make")}, false)

	if len(plan.updates) != 0 || plan.result.Unchanged != 1 {
		t.Fatalf("an unchanged Git file must not overwrite a panel edit: %+v", plan)
	}
}

func TestReconcileOverwritesPanelEditWhenGitChanged(t *testing.T) {
	edited := synced("a", "make")
	edited.Command = "make -j8"

	plan := reconcile([]models.JobDefinition{edited}, []*models.JobDefinition{fromGit("a", "make test")}, false)

	if len(plan.updates) != 1 || plan.result.Updated != 1 {
		t.Fatalf("want 1 update, got %+v", plan)
	}
	got := plan.updates[0]
	if got.Command != "make test" || got.GitState() != models.GitSynced {
		t.Errorf("got command %q state %q, want Git's version, synced", got.Command, got.GitState())
	}
}

func TestReconcileAdoptsManualDefinitionOnceCommitted(t *testing.T) {
	manual := models.JobDefinition{Slug: "a", JobSpec: spec("make")}

	plan := reconcile([]models.JobDefinition{manual}, []*models.JobDefinition{fromGit("a", "make")}, false)

	if len(plan.updates) != 1 || plan.updates[0].GitState() != models.GitSynced {
		t.Fatalf("committing a panel definition should mark it synced: %+v", plan)
	}
}

func TestReconcileDetachesMissingWhenPruneOff(t *testing.T) {
	manual := models.JobDefinition{Slug: "manual", JobSpec: spec("make")}
	current := []models.JobDefinition{synced("gone", "make"), manual}

	plan := reconcile(current, nil, false)

	if len(plan.deletes) != 0 {
		t.Fatalf("prune off must delete nothing, deleted %v", plan.deletes)
	}
	if plan.result.Detached != 1 || len(plan.updates) != 1 {
		t.Fatalf("want the Git definition detached, got %+v", plan)
	}
	if got := plan.updates[0].GitState(); got != models.GitRemoved {
		t.Errorf("detached state = %q, want removed", got)
	}
}

func TestReconcilePrunesEverythingNotInGit(t *testing.T) {
	manual := models.JobDefinition{Slug: "manual", JobSpec: spec("make")}
	current := []models.JobDefinition{synced("gone", "make"), manual, synced("kept", "make")}

	plan := reconcile(current, []*models.JobDefinition{fromGit("kept", "make")}, true)

	if strings.Join(plan.deletes, ",") != "gone,manual" || plan.result.Pruned != 2 {
		t.Fatalf("want gone and manual pruned, got %v", plan.deletes)
	}
}

func TestGitState(t *testing.T) {
	modified := synced("a", "make")
	modified.Command = "other"
	removed := models.JobDefinition{Slug: "a", SourcePath: "a.yaml"}

	for name, tc := range map[string]struct {
		def  models.JobDefinition
		want string
	}{
		"synced":   {synced("a", "make"), models.GitSynced},
		"modified": {modified, models.GitModified},
		"manual":   {models.JobDefinition{Slug: "a"}, models.GitManual},
		"removed":  {removed, models.GitRemoved},
	} {
		if got := tc.def.GitState(); got != tc.want {
			t.Errorf("%s: GitState() = %q, want %q", name, got, tc.want)
		}
	}
}

func TestParseDocsReadsMultipleDocuments(t *testing.T) {
	data := []byte("name: one\nruntime: alpine\ncommand: 'true'\n---\nname: Two Words\nruntime: alpine\ncommand: 'true'\n---\n")

	defs, err := parseDocs(data, "all.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 2 || defs[0].Slug != "one" || defs[1].Slug != "two-words" {
		t.Fatalf("got %+v", defs)
	}
	if defs[1].SourcePath != "all.yaml" || defs[1].Queue != "default" {
		t.Errorf("got path %q queue %q", defs[1].SourcePath, defs[1].Queue)
	}
}

func TestParseDocsRejectsWholeFileOnBadDocument(t *testing.T) {
	data := []byte("name: one\nruntime: alpine\ncommand: 'true'\n---\nruntime: alpine\n")

	if _, err := parseDocs(data, "bad.yaml"); err == nil {
		t.Fatal("want an error for a document with no name")
	}
}

// Export must produce YAML the sync reads back as the same spec — that is
// what makes copying the panel's config into Git lossless.
func TestExportRoundTrips(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "infrastructure", "dev", "jobdefs")
	files, skipped, err := loadDir(dir)
	if err != nil || len(skipped) > 0 {
		t.Fatalf("load dev jobdefs: %v %v", err, skipped)
	}
	if len(files) == 0 {
		t.Fatal("no dev job definitions found")
	}

	renamed := models.JobDefinition{
		Slug: "legacy-slug",
		JobSpec: models.JobSpec{
			Name: "Panel Job", Queue: "default", Runtime: "alpine",
			Command:     "echo hi\necho there",
			Description: "multi\nline",
			Parameters: []models.ParameterField{{
				Name: "count", Type: "number", Label: "Count", EnvVar: "COUNT", Default: 3.5,
			}},
		},
	}
	defs := []models.JobDefinition{renamed}
	for _, f := range files {
		defs = append(defs, *f)
	}

	out, err := Export(defs)
	if err != nil {
		t.Fatal(err)
	}
	back, err := parseDocs(out, "export.yaml")
	if err != nil {
		t.Fatalf("parse export: %v\n%s", err, out)
	}
	if len(back) != len(defs) {
		t.Fatalf("got %d documents, want %d", len(back), len(defs))
	}
	for i := range defs {
		if back[i].Slug != defs[i].Slug {
			t.Errorf("doc %d slug = %q, want %q", i, back[i].Slug, defs[i].Slug)
		}
		if !back[i].Equal(defs[i].JobSpec) {
			t.Errorf("doc %d (%s) did not round-trip\n%s", i, defs[i].Slug, out)
		}
	}
	if !strings.HasPrefix(string(out), "# legacy-slug.yaml\n") {
		t.Errorf("a panel definition should suggest <slug>.yaml, got:\n%s", out)
	}
}

func TestLoadDirSkipsDuplicateSlugs(t *testing.T) {
	dir := t.TempDir()
	doc := "name: same\nruntime: alpine\ncommand: 'true'\n"
	for _, name := range []string{"a.yaml", "b.yml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	defs, skipped, err := loadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) != 1 || defs[0].SourcePath != "a.yaml" || len(skipped) != 1 {
		t.Fatalf("got defs %+v skipped %v", defs, skipped)
	}
}
