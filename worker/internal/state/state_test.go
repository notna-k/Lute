package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := &State{WorkerID: "65a1", Secret: "lute_ws_x", Server: "core:50051"}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %o, want 600: the file holds the worker secret", mode)
	}
}

func TestLoadMissing(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "state.json"))
	if got != nil || err != nil {
		t.Errorf("Load of a missing file = %v, %v; want nil, nil", got, err)
	}
}

func TestLoadBroken(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"garbage.json":   "{not json",
		"no-secret.json": `{"worker_id":"65a1"}`,
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Errorf("%s: loaded without an error", name)
		}
	}
}

// A crash after the temp file was written but before the rename leaves a stray .tmp;
// the real file must be untouched and the next save must still work.
func TestSaveAfterCrashLeftTmp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	first := &State{WorkerID: "a", Secret: "s1"}
	if err := Save(path, first); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".state-123.tmp"), []byte(`{"worker_id":"half`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got.WorkerID != "a" {
		t.Fatalf("Load after a crash = %v, %v", got, err)
	}
	if err := Save(path, &State{WorkerID: "b", Secret: "s2"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := Load(path); got.WorkerID != "b" {
		t.Errorf("worker id = %q after the second save", got.WorkerID)
	}
}
