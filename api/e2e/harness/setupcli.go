//go:build e2e

package harness

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SetupResult is what the agent's `setup` subcommand reported.
type SetupResult struct {
	// WorkerID is the id core assigned the host.
	WorkerID string
	// DaemonPID is the background agent setup started, if it printed one.
	DaemonPID int
	// Output is everything setup wrote, for assertions and for diagnosing a failure.
	Output string
}

var (
	workerIDLine = regexp.MustCompile(`Worker ID:\s*([0-9a-fA-F]+)`)
	daemonPIDRe  = regexp.MustCompile(`Worker started \(PID (\d+)\)`)
)

// RunSetupCLI runs `lute-worker setup --claim-code …` the way an operator does: it
// answers the service-name prompt on stdin, and setup registers the host and starts
// the agent in the background. The background agent is detached from this process, so
// it is killed explicitly on cleanup.
func (s *Stack) RunSetupCLI(claimCode, serviceName string) SetupResult {
	s.t.Helper()

	if err := BuildWorker(); err != nil {
		s.t.Fatal(err)
	}

	// setup's background agent inherits this directory and writes its job logs under
	// it, so pointing it at a temp dir keeps the repo clean.
	workDir := s.t.TempDir()

	cmd := exec.Command(WorkerBinaryPath(), //nolint:gosec // the binary we just built
		"setup",
		"--api", s.BaseURL(),
		"--claim-code", claimCode,
	)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(serviceName + "\n")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	out, err := cmd.CombinedOutput()
	output := string(out)

	if f, ferr := os.Create(filepath.Join(s.ArtifactDir, "setup-cli.log")); ferr == nil { //nolint:gosec // derived from the test name
		_, _ = f.WriteString(output)
		_ = f.Close()
	}

	if err != nil {
		s.t.Fatalf("lute-worker setup failed: %v\noutput:\n%s", err, output)
	}

	res := SetupResult{Output: output}
	if m := workerIDLine.FindStringSubmatch(output); len(m) == 2 {
		res.WorkerID = m[1]
	}
	if m := daemonPIDRe.FindStringSubmatch(output); len(m) == 2 {
		res.DaemonPID, _ = strconv.Atoi(m[1])
	}

	if res.DaemonPID > 0 {
		pid := res.DaemonPID
		s.t.Cleanup(func() {
			// The daemon is in its own session; signal the process directly and give
			// it a moment, then insist.
			proc, err := os.FindProcess(pid)
			if err != nil {
				return
			}
			_ = proc.Signal(syscall.SIGTERM)
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if err := proc.Signal(syscall.Signal(0)); err != nil {
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
			_ = proc.Kill()
		})
	}
	return res
}
