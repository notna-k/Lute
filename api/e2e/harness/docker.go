//go:build e2e

// Package harness boots the pieces of Lute an end-to-end test needs: a Postgres
// instance, a core server, and real worker agents running as child processes.
//
// Nothing here fakes Lute's own behaviour. Core runs its production startup path
// against a throwaway database, agents are the compiled binary, and job bodies are
// real containers on the host Docker daemon. Tests observe the system only through
// HTTP, WebSocket and gRPC, so they survive refactors and fail on behaviour changes.
package harness

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// TestImages are pulled once per suite run so an individual test does not pay for
// an image pull inside its own timeout.
var TestImages = []string{"bash:5", "alpine:3"}

// DockerAvailable reports whether a Docker daemon is reachable.
func DockerAvailable() bool {
	return exec.Command("docker", "info").Run() == nil
}

var pullOnce sync.Once

// PullTestImages fetches the images job bodies run in. Safe to call repeatedly.
func PullTestImages() error {
	var err error
	pullOnce.Do(func() {
		for _, img := range TestImages {
			out, perr := exec.Command("docker", "pull", img).CombinedOutput()
			if perr != nil {
				err = fmt.Errorf("pull %s: %w: %s", img, perr, strings.TrimSpace(string(out)))
				return
			}
		}
	})
	return err
}

// ContainerSet is a snapshot of the containers on the host, used to tell the ones a
// suite run created from the ones that were already here.
type ContainerSet map[string]struct{}

// SnapshotContainers records every container currently on the host.
func SnapshotContainers() ContainerSet {
	out, err := docker("ps", "-aq", "--no-trunc")
	if err != nil {
		return nil
	}
	set := ContainerSet{}
	for line := range strings.SplitSeq(out, "\n") {
		if id := strings.TrimSpace(line); id != "" {
			set[id] = struct{}{}
		}
	}
	return set
}

// ReapJobContainers removes containers that appeared since the snapshot and run one of
// the suite's job images. An agent killed mid-build never runs its own cleanup, so
// without this every run that tests a crash leaves a container behind.
func ReapJobContainers(before ContainerSet) int {
	removed := 0
	for _, img := range TestImages {
		out, err := docker("ps", "-aq", "--no-trunc", "--filter", "ancestor="+img)
		if err != nil {
			continue
		}
		for line := range strings.SplitSeq(out, "\n") {
			id := strings.TrimSpace(line)
			if id == "" {
				continue
			}
			if _, existed := before[id]; existed {
				continue // not ours: it was here before the suite started
			}
			if _, err := docker("rm", "-f", id); err == nil {
				removed++
			}
		}
	}
	return removed
}

// docker runs a docker subcommand and returns its trimmed stdout.
func docker(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	out, err := cmd.Output()
	if err != nil {
		detail := ""
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			detail = ": " + strings.TrimSpace(string(ee.Stderr))
		}
		return "", fmt.Errorf("docker %s: %w%s", strings.Join(args, " "), err, detail)
	}
	return strings.TrimSpace(string(out)), nil
}
