package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

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
// System events are JSON-logged to job-{jobID}.log under logDir when logDir is set and also mirrored
// to the default slog logger (console). Container stdout lines go to the job file only (source=container).
// When logDir is empty, job file writes are discarded but system lines still appear on the default logger.
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

	jobLogger, closeLog, err := openJobLog(logDir, jobID)
	if err != nil {
		return 0, fmt.Errorf("job log: %w", err)
	}
	defer closeLog()

	dir, err := os.MkdirTemp("", "lute-job-*")
	if err != nil {
		return 0, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)
	logSystem(jobLogger, slog.LevelInfo, "temp dir", slog.String("path", dir))


	if err := cloneRepo(ctx, dir, spec.SourceRepository); err != nil {
		return 0, fmt.Errorf("clone: %w", err)
	}
	logSystem(jobLogger, slog.LevelInfo, "clone ok")


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

	exitCode, err := runContainer(runCtx, cli, dir, spec, jobLogger)
	if err != nil {
		return 0, err
	}
	if exitCode != 0 {
		return 0, fmt.Errorf("container exited with code %d", exitCode)
	}

	logSystem(jobLogger, slog.LevelInfo, "job finished successfully")
	return time.Since(start).Milliseconds(), nil
}
