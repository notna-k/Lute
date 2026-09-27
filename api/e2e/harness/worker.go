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

// workerBinaryName matches `make worker-build-linux`.
var workerBinaryName = fmt.Sprintf("lute-worker-%s-%s", runtime.GOOS, runtime.GOARCH)

var (
	buildOnce sync.Once
	buildErr  error
)

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func WorkerBinDir() string { return filepath.Join(repoRoot(), "worker", "bin") }

func WorkerBinaryPath() string { return filepath.Join(WorkerBinDir(), workerBinaryName) }

// BuildWorker compiles the agent once per run, so a bare `go test` works too.
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
		}
	})
	return buildErr
}

// Rootless reports that DOCKER_HOST points at a rootless engine (LUTE_E2E_ROOTLESS=1), so
// agents run without the rootful opt-in, as on a real host.
func Rootless() bool { return os.Getenv("LUTE_E2E_ROOTLESS") == "1" }

// Agent is one lute-worker process. It enrols itself from its environment, as the
// container does; WorkerID is known once it has connected.
type Agent struct {
	t        *testing.T
	stack    *Stack
	Name     string
	WorkerID string
	DataDir  string
	Queues   []string

	vars   []string
	cmd    *exec.Cmd
	stderr *syncBuffer
	exited chan struct{}
	waitMu sync.Mutex
	err    error
}

type AgentOption func(*agentOpts)

type agentOpts struct {
	queues      []string
	concurrency int
	dataDir     string
	vars        map[string]string
}

func WithQueues(queues ...string) AgentOption {
	return func(o *agentOpts) { o.queues = queues }
}

func WithConcurrency(n int) AgentOption {
	return func(o *agentOpts) { o.concurrency = n }
}

// WithDataDir reuses a data dir, e.g. to start an agent that is already registered.
func WithDataDir(dir string) AgentOption {
	return func(o *agentOpts) { o.dataDir = dir }
}

// WithVar sets, or with an empty value removes, a LUTE_* variable of the agent.
func WithVar(key, value string) AgentOption {
	return func(o *agentOpts) { o.vars[key] = value }
}

// StartAgent runs lute-worker named name with the stack's bootstrap token; it does not wait.
func (s *Stack) StartAgent(name string, options ...AgentOption) *Agent {
	s.t.Helper()
	if err := BuildWorker(); err != nil {
		s.t.Fatal(err)
	}
	opts := &agentOpts{queues: []string{"default"}, concurrency: 1, vars: map[string]string{}}
	for _, o := range options {
		o(opts)
	}
	if opts.dataDir == "" {
		opts.dataDir = s.t.TempDir()
	}

	vars := map[string]string{
		"LUTE_SERVER":      s.GRPCAddr(),
		"LUTE_TOKEN":       BootstrapToken,
		"LUTE_NAME":        name,
		"LUTE_DATA_DIR":    opts.dataDir,
		"LUTE_QUEUES":      strings.Join(opts.queues, ","),
		"LUTE_CONCURRENCY": strconv.Itoa(opts.concurrency),
	}
	if !Rootless() {
		vars["LUTE_ALLOW_ROOTFUL"] = "1" // CI runners and dev machines run rootful Docker
	}
	for k, v := range opts.vars {
		vars[k] = v
	}
	all := os.Environ()
	for k, v := range vars {
		if v != "" {
			all = append(all, k+"="+v)
		}
	}

	a := &Agent{t: s.t, stack: s, Name: name, DataDir: opts.dataDir, Queues: opts.queues, vars: all}
	a.start()
	return a
}

func (a *Agent) start() {
	s := a.stack
	a.stderr = &syncBuffer{}
	a.exited = make(chan struct{})
	a.cmd = exec.Command(WorkerBinaryPath(), "run") //nolint:gosec // path is the binary we just built
	a.cmd.Env = a.vars
	// Own process group, so Kill takes down the agent and anything it spawned.
	a.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	artifact := filepath.Join(s.ArtifactDir, fmt.Sprintf("agent-%s-%d.log", SafeName(a.Name), len(s.agents)))
	if f, err := os.Create(artifact); err == nil { //nolint:gosec // derived from the test name
		a.stderr.tee = f
		s.t.Cleanup(func() { _ = f.Close() })
	}
	a.cmd.Stdout = a.stderr
	a.cmd.Stderr = a.stderr

	if err := a.cmd.Start(); err != nil {
		s.t.Fatalf("start agent %s: %v", a.Name, err)
	}
	go func() {
		a.err = a.cmd.Wait()
		close(a.exited)
	}()
	s.agents = append(s.agents, a)
	s.t.Cleanup(a.stopQuietly)
}

// Restart starts the same agent again, with the same data dir and so the same identity.
func (a *Agent) Restart() *Agent {
	a.t.Helper()
	if !a.Exited() {
		a.t.Fatalf("agent %s is still running", a.Name)
	}
	again := &Agent{t: a.t, stack: a.stack, Name: a.Name, WorkerID: a.WorkerID, DataDir: a.DataDir, Queues: a.Queues, vars: a.vars}
	again.start()
	return again
}

// Kill takes the agent down without warning, like a crashed build host.
func (a *Agent) Kill() {
	a.t.Helper()
	a.signal(syscall.SIGKILL)
	a.WaitExit(10 * time.Second)
}

// Terminate sends SIGTERM, as `docker stop` does; it does not wait.
func (a *Agent) Terminate() { a.signal(syscall.SIGTERM) }

func (a *Agent) WaitExit(timeout time.Duration) {
	a.t.Helper()
	select {
	case <-a.exited:
	case <-time.After(timeout):
		a.t.Fatalf("agent %s still running after %s\nstderr:\n%s", a.Name, timeout, a.Stderr())
	}
}

func (a *Agent) Exited() bool {
	select {
	case <-a.exited:
		return true
	default:
		return false
	}
}

func (a *Agent) Stderr() string { return a.stderr.String() }

// ExitCode is the process's exit status, or -1 while it runs.
func (a *Agent) ExitCode() int {
	if !a.Exited() || a.cmd.ProcessState == nil {
		return -1
	}
	return a.cmd.ProcessState.ExitCode()
}

// ExitError is only meaningful once Exited reports true.
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
	// SIGKILL takes the whole group; other signals go to the agent alone, as Docker sends them.
	if sig == syscall.SIGKILL {
		if err := syscall.Kill(-a.cmd.Process.Pid, sig); err == nil {
			return
		}
	}
	_ = a.cmd.Process.Signal(sig)
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

// syncBuffer collects a child's output while a test reads it, and mirrors it to an artifact file.
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
