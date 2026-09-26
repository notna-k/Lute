// Package datadir owns the agent's host directory: identity, job logs and job metadata.
//
//	<root>/.lute-data           marker: {"version":1}
//	<root>/state.json           identity (0600)
//	<root>/jobs/<id>/log        the job's log
//	<root>/jobs/<id>/meta.json  what ran and how it ended
package datadir

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lute/worker/internal/joblog"
)

const (
	markerName    = ".lute-data"
	markerVersion = 1
)

type Dir struct {
	Root string
}

// Open checks root is usable and claims it with a marker. With requireMount, as in the
// image, root must be a mount point, so state never lives in the container's own layer.
func Open(root string, requireMount bool) (*Dir, error) {
	root = filepath.Clean(root)
	if requireMount {
		f, err := os.Open("/proc/self/mountinfo")
		if err != nil {
			return nil, fmt.Errorf("read mount table: %w", err)
		}
		mounted, err := IsMountPoint(f, root)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("read mount table: %w", err)
		}
		if !mounted {
			return nil, fmt.Errorf("%s is not mounted. Add -v ~/.local/share/lute-worker:%s", root, root)
		}
	} else if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create %s: %w", root, err)
	}

	probe := filepath.Join(root, ".probe")
	if err := os.WriteFile(probe, nil, 0o600); err != nil {
		return nil, fmt.Errorf("%s is not writable by uid %d: %w", root, os.Getuid(), err)
	}
	_ = os.Remove(probe)

	if err := claim(root); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "jobs"), 0o700); err != nil {
		return nil, err
	}
	return &Dir{Root: root}, nil
}

// claim writes the marker into an empty dir, and refuses one that holds other files but no
// marker: that is a wrong mount, such as $HOME.
func claim(root string) error {
	marker := filepath.Join(root, markerName)
	raw, err := os.ReadFile(marker) //nolint:gosec // inside the data dir
	if err == nil {
		var m struct {
			Version int `json:"version"`
		}
		if err := json.Unmarshal(raw, &m); err != nil || m.Version != markerVersion {
			return fmt.Errorf("%s has an unknown marker %s; is this a Lute worker data dir?", root, markerName)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s holds other files but no %s marker; mount an empty directory for the worker", root, markerName)
	}
	return os.WriteFile(marker, []byte(`{"version":1}`+"\n"), 0o600)
}

func (d *Dir) StatePath() string { return filepath.Join(d.Root, "state.json") }

// JobDir creates and returns the job's directory.
func (d *Dir) JobDir(jobID string) (string, error) {
	dir, err := joblog.Dir(d.Root, jobID)
	if err != nil {
		return "", err
	}
	return dir, os.MkdirAll(dir, 0o700)
}

// Meta is jobs/<id>/meta.json: written when a job starts and again when it ends.
type Meta struct {
	JobID       string     `json:"job_id"`
	WorkerID    string     `json:"worker_id"`
	Image       string     `json:"image"`
	ImageDigest string     `json:"image_digest,omitempty"`
	Repository  string     `json:"repository,omitempty"`
	Commit      string     `json:"commit,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	ExitCode    *int64     `json:"exit_code,omitempty"`
	Error       string     `json:"error"`
}

func (d *Dir) WriteMeta(m *Meta) error {
	dir, err := d.JobDir(m.JobID)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, ".meta.json.tmp")
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "meta.json"))
}

// Prune removes job dirs last modified before now-retention and returns how many it removed.
func (d *Dir) Prune(retention time.Duration, now time.Time) (int, error) {
	jobs := filepath.Join(d.Root, "jobs")
	entries, err := os.ReadDir(jobs)
	if err != nil {
		return 0, err
	}
	cutoff := now.Add(-retention)
	removed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(jobs, e.Name())); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// IsMountPoint reports whether path is a mount point in a /proc/<pid>/mountinfo table.
func IsMountPoint(mountinfo io.Reader, path string) (bool, error) {
	path = filepath.Clean(path)
	sc := bufio.NewScanner(mountinfo)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		// id parent major:minor root mount-point options ...
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 {
			continue
		}
		if filepath.Clean(unescape(fields[4])) == path {
			return true, nil
		}
	}
	return false, sc.Err()
}

// unescape decodes the octal escapes mountinfo uses for space, tab, newline and backslash.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
