//go:build e2e

// Package e2e drives Lute the way its users and its build hosts do: HTTP and
// WebSocket in from the panel, real agent processes over gRPC, real containers for
// job bodies, and assertions only on what those surfaces report back.
//
// Run with: make e2e
package e2e

import (
	"fmt"
	"os"
	"testing"

	"github.com/lute/api/e2e/harness"
)

// pg is the database server every stack carves a database out of.
var pg *harness.Postgres

func TestMain(m *testing.M) {
	code, err := run(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e setup:", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func run(m *testing.M) (int, error) {
	if !harness.DockerAvailable() && os.Getenv("LUTE_E2E_POSTGRES_DSN") == "" {
		return 0, fmt.Errorf("this suite needs a Docker daemon, or LUTE_E2E_POSTGRES_DSN pointing at a Postgres")
	}

	// Build the agent and fetch the job images up front: neither belongs inside an
	// individual test's timeout.
	if err := harness.BuildWorker(); err != nil {
		return 0, err
	}
	if harness.DockerAvailable() {
		if err := harness.PullTestImages(); err != nil {
			return 0, err
		}
	}

	// Containers already on this machine are none of the suite's business; the ones
	// that appear from here on are, including those a killed agent left behind.
	existing := harness.SnapshotContainers()

	started, err := harness.StartPostgres()
	if err != nil {
		return 0, err
	}
	pg = started

	code := m.Run()

	pg.Stop()
	if harness.DockerAvailable() {
		if n := harness.ReapJobContainers(existing); n > 0 {
			fmt.Fprintf(os.Stderr, "e2e: removed %d job container(s) left by killed agents\n", n)
		}
	}
	return code, nil
}
