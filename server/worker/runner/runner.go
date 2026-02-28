package runner

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// Spec is the container job specification (mirrors proto ContainerJobSpec).
// JobAssignment.payload for type "container" is JSON matching this struct.
type Spec struct {
	SourceRepository string            `json:"source_repository"`
	Runtime          string            `json:"runtime"`
	RequestParams    map[string]string `json:"request_params"`
	Command          string            `json:"command"`
}

const workspaceMount = "/workspace"
const commandScriptName = "_user_command.sh"

// Run clones the repo, writes the command script, runs a Docker container, and cleans up.
// Returns elapsed milliseconds and an error on failure.
func Run(ctx context.Context, spec *Spec, timeoutSec int32) (elapsedMs int64, err error) {
	start := time.Now()
	defer func() {
		elapsedMs = time.Since(start).Milliseconds()
	}()

	if spec == nil {
		return 0, fmt.Errorf("spec is nil")
	}
	if err := validateGitHubRepo(spec.SourceRepository); err != nil {
		return 0, fmt.Errorf("invalid source_repository: %w", err)
	}
	if spec.Runtime == "" {
		return 0, fmt.Errorf("runtime (Docker image) is required")
	}

	dir, err := os.MkdirTemp("", "lute-job-*")
	if err != nil {
		return 0, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := cloneRepo(ctx, dir, spec.SourceRepository); err != nil {
		return 0, fmt.Errorf("clone: %w", err)
	}

	scriptPath := filepath.Join(dir, commandScriptName)
	if err := os.WriteFile(scriptPath, []byte(spec.Command), 0700); err != nil {
		return 0, fmt.Errorf("write command script: %w", err)
	}

	runCtx := ctx
	if timeoutSec > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return 0, fmt.Errorf("docker client: %w", err)
	}
	defer cli.Close()

	bind := dir + ":" + workspaceMount
	env := envFromParams(spec.RequestParams)
	cfg := &container.Config{
		Image: spec.Runtime,
		Cmd:   []string{"bash", filepath.Join(workspaceMount, commandScriptName)},
		Env:   env,
	}
	hostConfig := &container.HostConfig{
		Binds: []string{bind},
	}

	createResp, err := cli.ContainerCreate(runCtx, cfg, hostConfig, nil, nil, "")
	if err != nil {
		return 0, fmt.Errorf("container create: %w", err)
	}
	containerID := createResp.ID
	defer func() {
		_ = cli.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
	}()

	if err := cli.ContainerStart(runCtx, containerID, container.StartOptions{}); err != nil {
		return 0, fmt.Errorf("container start: %w", err)
	}

	statusCh, errCh := cli.ContainerWait(runCtx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return 0, fmt.Errorf("container wait: %w", err)
		}
	case res := <-statusCh:
		if res.StatusCode != 0 {
			return 0, fmt.Errorf("container exited with code %d", res.StatusCode)
		}
	case <-runCtx.Done():
		return 0, fmt.Errorf("timeout after %ds", timeoutSec)
	}

	return time.Since(start).Milliseconds(), nil
}

func validateGitHubRepo(repoURL string) error {
	if repoURL == "" {
		return fmt.Errorf("source_repository is required")
	}
	u, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("only https is allowed (got %q)", u.Scheme)
	}
	host := strings.ToLower(u.Host)
	if host != "github.com" && host != "www.github.com" {
		return fmt.Errorf("only github.com is allowed (got %q)", u.Host)
	}
	return nil
}

func cloneRepo(ctx context.Context, dir, repoURL string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func envFromParams(params map[string]string) []string {
	if len(params) == 0 {
		return nil
	}
	out := make([]string, 0, len(params))
	for k, v := range params {
		out = append(out, k+"="+v)
	}
	return out
}
