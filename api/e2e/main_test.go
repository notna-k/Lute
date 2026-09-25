//go:build e2e

// Package e2e drives Lute end to end through its public surfaces. Run with: make e2e
package e2e

import (
	"fmt"
	"os"
	"testing"

	"github.com/lute/api/e2e/harness"
	"github.com/lute/api/internal/testutil/pgtest"
)

var pg *pgtest.Server

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

	// Done up front so no test pays for them inside its timeout.
	if err := harness.BuildWorker(); err != nil {
		return 0, err
	}
	if harness.DockerAvailable() {
		if err := harness.PullTestImages(); err != nil {
			return 0, err
		}
	}

	// Only containers created from here on, e.g. by a killed agent, are the suite's to reap.
	existing := harness.SnapshotContainers()

	started, err := pgtest.Start()
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
