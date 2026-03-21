package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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

// execLog writes a system/execution log line to both the process logger and,
// when an execution log file is provided, to that file.
func execLog(execFile *os.File, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	if execFile != nil {
		fmt.Fprintln(execFile, msg)
	}
}

// Run clones the repo, writes the command script, runs a Docker container, and cleans up.
// When logDir is non-empty, container stdout/stderr are written to logs-{jobID}
// and system/execution messages to execution-{jobID} under logDir.
// Returns elapsed milliseconds and an error on failure.
func Run(ctx context.Context, jobID, logDir string, spec *Spec, timeoutSec int32) (elapsedMs int64, err error) {
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

	var execFile *os.File
	if logDir != "" {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return 0, fmt.Errorf("create log dir: %w", err)
		}
		f, err := os.Create(filepath.Join(logDir, "execution-"+jobID))
		if err != nil {
			return 0, fmt.Errorf("create execution log: %w", err)
		}
		execFile = f
		defer execFile.Close()
	}

	dir, err := os.MkdirTemp("", "lute-job-*")
	if err != nil {
		return 0, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)
	execLog(execFile, "[container] temp dir: %s", dir)

	if err := cloneRepo(ctx, dir, spec.SourceRepository); err != nil {
		return 0, fmt.Errorf("clone: %w", err)
	}
	execLog(execFile, "[container] clone ok")

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
	execLog(execFile, "[container] pulling image %s", spec.Runtime)

	pullResp, err := cli.ImagePull(runCtx, spec.Runtime, image.PullOptions{})
	if err != nil {
		return 0, fmt.Errorf("image pull %q: %w", spec.Runtime, err)
	}
	_, _ = io.Copy(io.Discard, pullResp)
	_ = pullResp.Close()
	execLog(execFile, "[container] image pull ok")

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
	execLog(execFile, "[container] container created: %s", containerID[:12])
	defer func() {
		_ = cli.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
	}()

	if err := cli.ContainerStart(runCtx, containerID, container.StartOptions{}); err != nil {
		return 0, fmt.Errorf("container start: %w", err)
	}
	execLog(execFile, "[container] container started, waiting for exit")

	statusCh, errCh := cli.ContainerWait(runCtx, containerID, container.WaitConditionNotRunning)
	var exitCode int64
	select {
	case err := <-errCh:
		if err != nil {
			return 0, fmt.Errorf("container wait: %w", err)
		}
	case res := <-statusCh:
		exitCode = res.StatusCode
	case <-runCtx.Done():
		return 0, fmt.Errorf("timeout after %ds", timeoutSec)
	}

	// Capture container stdout/stderr
	logReader, err := cli.ContainerLogs(context.Background(), containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err == nil {
		var outBuf, errBuf bytes.Buffer
		_, _ = stdcopy.StdCopy(&outBuf, &errBuf, logReader)
		_ = logReader.Close()

		if outBuf.Len() > 0 {
			execLog(execFile, "[container] stdout:\n%s", strings.TrimSuffix(outBuf.String(), "\n"))
		}
		if errBuf.Len() > 0 {
			execLog(execFile, "[container] stderr:\n%s", strings.TrimSuffix(errBuf.String(), "\n"))
		}

		if logDir != "" {
			logsPath := filepath.Join(logDir, "logs-"+jobID)
			var combined bytes.Buffer
			if outBuf.Len() > 0 {
				combined.WriteString(outBuf.String())
			}
			if errBuf.Len() > 0 {
				if combined.Len() > 0 {
					combined.WriteString("\n")
				}
				combined.WriteString(errBuf.String())
			}
			_ = os.WriteFile(logsPath, combined.Bytes(), 0644)
		}
	}

	if exitCode != 0 {
		return 0, fmt.Errorf("container exited with code %d", exitCode)
	}

	execLog(execFile, "[container] job finished successfully")

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
