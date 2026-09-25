//go:build e2e

// Package harness boots real core, real agent processes and real containers; tests observe them only over HTTP, WebSocket and gRPC.
package harness

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// TestImages are pulled once up front, so no test pays for a pull inside its timeout.
var TestImages = []string{"bash:5", "alpine:3"}

func DockerAvailable() bool {
	return exec.Command("docker", "info").Run() == nil
}

var pullOnce sync.Once

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

// ContainerSet tells the containers a suite run created from the ones already on the host.
type ContainerSet map[string]struct{}

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

// ReapJobContainers removes job containers created since the snapshot: an agent killed
// mid-build never cleans up after itself.
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
