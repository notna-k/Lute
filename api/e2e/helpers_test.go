//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lute/api/e2e/harness"
)

// newStack boots a Lute with the suite's job definitions already committed, which is
// the state an operator finds after a deploy: definitions synced from Git, no builds
// yet, no agents connected.
func newStack(t *testing.T, opts ...harness.StackOption) *harness.Stack {
	t.Helper()
	return harness.NewStack(t, pg, append([]harness.StackOption{
		harness.WithJobDefsDir(jobDefsDir(t, allJobDefs)),
	}, opts...)...)
}

// newBareStack boots a Lute with no definitions, for tests that write their own.
func newBareStack(t *testing.T, opts ...harness.StackOption) *harness.Stack {
	t.Helper()
	return harness.NewStack(t, pg, append([]harness.StackOption{
		harness.WithJobDefsDir(jobDefsDir(t, nil)),
	}, opts...)...)
}

// jobDefsDir writes definition files into a directory core will sync from at boot.
func jobDefsDir(t *testing.T, defs map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range defs {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// slugsOf lists the slugs of a definition set, for set comparisons.
func slugsOf(defs []harness.JobDefinition) []string {
	out := make([]string, 0, len(defs))
	for _, d := range defs {
		out = append(out, d.Slug)
	}
	return out
}

// findDef returns the definition with the given slug, or fails.
func findDef(t *testing.T, defs []harness.JobDefinition, slug string) harness.JobDefinition {
	t.Helper()
	for _, d := range defs {
		if d.Slug == slug {
			return d
		}
	}
	t.Fatalf("no definition %q among %v", slug, slugsOf(defs))
	return harness.JobDefinition{}
}

// jobIDOf resolves a build's queue-job id, which is what the queue and log APIs are
// keyed by. The panel addresses a build by run id; the two are deliberately distinct.
func jobIDOf(t *testing.T, c *harness.Client, slug, runID string) string {
	t.Helper()
	builds, err := c.Builds(slug)
	if err != nil {
		t.Fatalf("list builds of %s: %v", slug, err)
	}
	for _, b := range builds {
		if b.RunID == runID {
			return b.JobID
		}
	}
	t.Fatalf("no build %s of %s", runID, slug)
	return ""
}

// containerPayload builds the payload a "container" job carries, for the tests that
// queue work directly rather than through a definition.
func containerPayload(t *testing.T, runtime, command string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(harness.ContainerSpec{Runtime: runtime, Command: command})
	if err != nil {
		t.Fatalf("marshal container spec: %v", err)
	}
	return raw
}

// linesContain reports whether any line contains the substring.
func linesContain(lines []string, want string) bool {
	for _, l := range lines {
		if strings.Contains(l, want) {
			return true
		}
	}
	return false
}
