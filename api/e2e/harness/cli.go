//go:build e2e

package harness

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	cliOnce sync.Once
	cliErr  error
)

// CLIPath is the built lute command, run under Node as npm installs it.
func CLIPath() string { return filepath.Join(repoRoot(), "cli", "dist", "main.js") }

// BuildCLI installs the CLI's dependencies and compiles it, once per suite run. Only the
// tests that drive lute call it, so the rest of the suite needs no Node.
func BuildCLI(t *testing.T) {
	t.Helper()
	cliOnce.Do(func() {
		if _, err := exec.LookPath("npm"); err != nil {
			cliErr = errors.New("npm is not on PATH; the CLI tests need Node 22.18 or newer")
			return
		}
		dir := filepath.Join(repoRoot(), "cli")
		for _, args := range [][]string{{"ci", "--ignore-scripts", "--no-audit", "--no-fund"}, {"run", "build"}} {
			cmd := exec.Command("npm", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				cliErr = fmt.Errorf("npm %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
				return
			}
		}
	})
	if cliErr != nil {
		t.Fatalf("build the CLI: %v", cliErr)
	}
}

// CLI runs lute as one user of one machine: its own config directory, never the real
// keychain, and only the environment given here.
type CLI struct {
	t         *testing.T
	ConfigDir string
	Env       []string
}

func (s *Stack) CLI() *CLI {
	BuildCLI(s.t)
	return &CLI{
		t:         s.t,
		ConfigDir: s.t.TempDir(),
		Env:       []string{"LUTE_NO_KEYCHAIN=1", "LUTE_POLL_INTERVAL_MS=200", "NO_COLOR=1"},
	}
}

// With returns a copy that also sets these KEY=value pairs.
func (c *CLI) With(vars ...string) *CLI {
	clone := *c
	clone.Env = append(append([]string(nil), c.Env...), vars...)
	return &clone
}

type CLIResult struct {
	Code   int
	Stdout string
	Stderr string
}

func (r CLIResult) String() string {
	return fmt.Sprintf("exit %d\nstdout:\n%s\nstderr:\n%s", r.Code, r.Stdout, r.Stderr)
}

// Run executes lute with args and stdin; a non-zero exit is a result, not an error.
func (c *CLI) Run(stdin string, args ...string) CLIResult {
	c.t.Helper()
	cmd := exec.Command("node", append([]string{CLIPath()}, args...)...)
	cmd.Env = append([]string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + c.ConfigDir,
		"LUTE_CONFIG_DIR=" + c.ConfigDir,
	}, c.Env...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := CLIResult{Stdout: stdout.String(), Stderr: stderr.String()}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr):
		res.Code = exitErr.ExitCode()
	default:
		c.t.Fatalf("run lute %s: %v", strings.Join(args, " "), err)
	}
	return res
}
