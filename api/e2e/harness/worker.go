//go:build e2e

package harness

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// workerBinaryName is what `make worker-build-linux` produces. Keeping the same
// name means core's binary-serving endpoints index it the way they do in production.
var workerBinaryName = fmt.Sprintf("lute-worker-%s-%s", runtime.GOOS, runtime.GOARCH)

var (
	buildOnce sync.Once
	buildErr  error
)

// repoRoot walks up from this file to the module's parent, which is the repo root.
func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	// .../api/e2e/harness/worker.go -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// WorkerBinDir is where the worker binary and its VERSION file live.
func WorkerBinDir() string { return filepath.Join(repoRoot(), "worker", "bin") }

// WorkerBinaryPath is the agent the suite runs.
func WorkerBinaryPath() string { return filepath.Join(WorkerBinDir(), workerBinaryName) }

// BuildWorker compiles the agent once per suite run. `make e2e` builds it up front;
// this keeps a bare `go test` working, and never rebuilds inside a test's timeout.
func BuildWorker() error {
	buildOnce.Do(func() {
		if _, err := os.Stat(WorkerBinaryPath()); err == nil && os.Getenv("LUTE_E2E_REBUILD_WORKER") == "" {
			return
		}
		cmd := exec.Command("go", "build", "-o", WorkerBinaryPath(), "./cmd/worker")
		cmd.Dir = filepath.Join(repoRoot(), "worker")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("build worker: %w: %s", err, strings.TrimSpace(string(out)))
			return
		}
		versionFile := filepath.Join(WorkerBinDir(), "VERSION")
		if _, err := os.Stat(versionFile); err != nil {
			buildErr = os.WriteFile(versionFile, []byte("e2e\n"), 0o600)
		}
	})
	return buildErr
}

// Agent is a worker agent running as a child process, as on a real build host.
type Agent struct {
	t        *testing.T
	WorkerID string
	Queues   []string
	LogsDir  string

	cmd    *exec.Cmd
	stderr *syncBuffer
	exited chan struct{}
	waitMu sync.Mutex
	err    error
}

// AgentOption configures the agent's command line.
type AgentOption func(*agentOpts)

type agentOpts struct {
	queues      []string
	concurrency int
	logsDir     string
}

// WithQueues sets the queues the agent pulls work for.
func WithQueues(queues ...string) AgentOption {
	return func(o *agentOpts) { o.queues = queues }
}

// WithConcurrency caps how many builds the agent runs at once.
func WithConcurrency(n int) AgentOption {
	return func(o *agentOpts) { o.concurrency = n }
}

// StartAgent runs `lute-worker run` against this stack and stops it on cleanup.
func (s *Stack) StartAgent(workerID string, options ...AgentOption) *Agent {
	s.t.Helper()

	if err := BuildWorker(); err != nil {
		s.t.Fatal(err)
	}

	opts := &agentOpts{queues: []string{"default"}, concurrency: 1}
	for _, o := range options {
		o(opts)
	}
	if opts.logsDir == "" {
		opts.logsDir = s.t.TempDir()
	}

	a := &Agent{
		t:        s.t,
		WorkerID: workerID,
		Queues:   opts.queues,
		LogsDir:  opts.logsDir,
		stderr:   &syncBuffer{},
		exited:   make(chan struct{}),
	}

	a.cmd = exec.Command(WorkerBinaryPath(), //nolint:gosec // path is the binary we just built
		"run",
		"--server", s.GRPCAddr(),
		"--worker-id", workerID,
		"--queues", strings.Join(opts.queues, ","),
		"--concurrency", strconv.Itoa(opts.concurrency),
		"--job-logs-dir", opts.logsDir,
	)
	a.cmd.Env = os.Environ()
	// Own process group, so Kill takes down the agent and anything it spawned.
	a.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	artifact := filepath.Join(s.ArtifactDir, "agent-"+shortID(workerID)+".log")
	if f, err := os.Create(artifact); err == nil { //nolint:gosec // derived from the test name
		a.stderr.tee = f
		s.t.Cleanup(func() { _ = f.Close() })
	}
	a.cmd.Stdout = a.stderr
	a.cmd.Stderr = a.stderr

	if err := a.cmd.Start(); err != nil {
		s.t.Fatalf("start agent %s: %v", workerID, err)
	}
	go func() {
		a.err = a.cmd.Wait()
		close(a.exited)
	}()

	s.agents = append(s.agents, a)
	s.t.Cleanup(a.stopQuietly)
	return a
}

// Kill takes the agent down without warning — a crashed build host. The build it was
// running is now core's problem to notice.
func (a *Agent) Kill() {
	a.t.Helper()
	a.signal(syscall.SIGKILL)
	a.WaitExit(10 * time.Second)
}

// WaitExit fails the test if the agent has not exited within timeout.
func (a *Agent) WaitExit(timeout time.Duration) {
	a.t.Helper()
	select {
	case <-a.exited:
	case <-time.After(timeout):
		a.t.Fatalf("agent %s still running after %s\nstderr:\n%s", a.WorkerID, timeout, a.Stderr())
	}
}

// Exited reports whether the agent process has finished.
func (a *Agent) Exited() bool {
	select {
	case <-a.exited:
		return true
	default:
		return false
	}
}

// Stderr is everything the agent has logged so far.
func (a *Agent) Stderr() string { return a.stderr.String() }

// ExitError is why the process ended: nil for a clean exit, non-nil for a crash or a
// non-zero status. Only meaningful once Exited reports true, which is what publishes it.
func (a *Agent) ExitError() error {
	if !a.Exited() {
		return nil
	}
	return a.err
}

func (a *Agent) signal(sig syscall.Signal) {
	if a.cmd == nil || a.cmd.Process == nil || a.Exited() {
		return
	}
	// Negative pid signals the whole group: the agent plus any child it started.
	if err := syscall.Kill(-a.cmd.Process.Pid, sig); err != nil {
		_ = a.cmd.Process.Signal(sig)
	}
}

func (a *Agent) stopQuietly() {
	a.waitMu.Lock()
	defer a.waitMu.Unlock()
	if a.Exited() {
		return
	}
	a.signal(syscall.SIGKILL)
	select {
	case <-a.exited:
	case <-time.After(5 * time.Second):
	}
}

// syncBuffer collects a child process's output safely from the reader goroutine
// while a test reads it, and mirrors it to an artifact file.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
	tee *os.File
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tee != nil {
		_, _ = b.tee.Write(p)
	}
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func shortID(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
