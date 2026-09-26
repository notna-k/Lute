package datadir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A trimmed mountinfo from a container: overlay root, bind-mounted data dir and socket,
// a nested path, and a mount point with a space (octal-escaped as \040).
const mountinfo = `1265 1180 0:112 / / rw,relatime master:520 - overlay overlay rw,lowerdir=/var/lib/docker/overlay2/l/A
1266 1265 0:115 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
1271 1265 259:2 /home/ci/.local/share/lute-worker /var/lib/lute-worker rw,relatime - ext4 /dev/nvme0n1p2 rw
1272 1265 259:2 /home/ci/.local/share/docker/containers/7f3c2d9e8b1a4c6d5e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d/hostname /etc/hostname rw,relatime - ext4 /dev/nvme0n1p2 rw
1273 1265 0:26 /user/1000/docker.sock /var/run/docker.sock rw,nosuid,nodev - tmpfs tmpfs rw
1274 1271 259:2 /home/ci/cache /var/lib/lute-worker/cache rw,relatime - ext4 /dev/nvme0n1p2 rw
1275 1265 259:2 /data /mnt/with\040space rw,relatime - ext4 /dev/nvme0n1p2 rw
`

func TestIsMountPoint(t *testing.T) {
	tests := map[string]bool{
		"/var/lib/lute-worker":       true,
		"/var/lib/lute-worker/":      true,
		"/var/lib/lute-worker/cache": true,
		"/mnt/with space":            true,
		"/var/lib":                   false,
		"/var/lib/lute-worker/jobs":  false,
		"/var/lib/lute":              false,
	}
	for path, want := range tests {
		got, err := IsMountPoint(strings.NewReader(mountinfo), path)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("IsMountPoint(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestOpenClaimsAnEmptyDir(t *testing.T) {
	root := filepath.Join(t.TempDir(), "data")
	d, err := Open(root, false)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, markerName))
	if err != nil {
		t.Fatalf("no marker written: %v", err)
	}
	var m map[string]int
	if err := json.Unmarshal(raw, &m); err != nil || m["version"] != 1 {
		t.Errorf("marker = %s", raw)
	}
	if _, err := os.Stat(filepath.Join(root, "jobs")); err != nil {
		t.Errorf("jobs dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".probe")); !os.IsNotExist(err) {
		t.Error("the write probe was left behind")
	}
	// Opening again finds its own marker next to its own files.
	if err := os.WriteFile(d.StatePath(), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root, false); err != nil {
		t.Errorf("reopen: %v", err)
	}
}

func TestOpenRefusesAForeignDir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".bashrc"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Open(root, false)
	if err == nil || !strings.Contains(err.Error(), "marker") {
		t.Errorf("err = %v, want a refusal naming the marker", err)
	}
}

func TestOpenRefusesAnUnknownMarker(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, markerName), []byte(`{"version":9}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root, false); err == nil {
		t.Error("a marker from an unknown version was accepted")
	}
}

func TestOpenRequiresAMount(t *testing.T) {
	_, err := Open(t.TempDir(), true)
	if err == nil || !strings.Contains(err.Error(), "is not mounted") {
		t.Errorf("err = %v, want the not-mounted message", err)
	}
}

func TestOpenRefusesAReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root writes anywhere")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })
	if _, err := Open(root, false); err == nil || !strings.Contains(err.Error(), "not writable") {
		t.Errorf("err = %v, want not writable", err)
	}
}

func TestWriteMetaAndPrune(t *testing.T) {
	d, err := Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	code := int64(0)
	if err := d.WriteMeta(&Meta{JobID: "old", Image: "bash:5", ExitCode: &code}); err != nil {
		t.Fatal(err)
	}
	if err := d.WriteMeta(&Meta{JobID: "new", Image: "bash:5"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(d.Root, "jobs", "old", "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got Meta
	if err := json.Unmarshal(raw, &got); err != nil || got.Image != "bash:5" || got.ExitCode == nil {
		t.Errorf("meta.json = %s", raw)
	}

	now := time.Now()
	old := now.Add(-48 * time.Hour)
	if err := os.Chtimes(filepath.Join(d.Root, "jobs", "old"), old, old); err != nil {
		t.Fatal(err)
	}
	n, err := d.Prune(24*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("pruned %d, want 1", n)
	}
	if _, err := os.Stat(filepath.Join(d.Root, "jobs", "new")); err != nil {
		t.Errorf("a recent job was pruned: %v", err)
	}
}

func TestJobDirRejectsEscapes(t *testing.T) {
	d, err := Open(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "..", "a/../../b", "x/y"} {
		if _, err := d.JobDir(id); err == nil {
			t.Errorf("JobDir(%q) was accepted", id)
		}
	}
}
