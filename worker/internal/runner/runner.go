package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/client"
)

// Spec is the JSON payload of a "container" job (mirrors proto ContainerJobSpec).
type Spec struct {
	SourceRepository string            `json:"source_repository"`
	Runtime          string            `json:"runtime"`
	RequestParams    map[string]string `json:"request_params"`
	Command          string            `json:"command"`
}

const (
	workspaceMount    = "/workspace"
	commandScriptName = "_user_command.sh"
)

// Run clones the optional repository into a temp workspace and runs spec.Command in a container.
// Container stdout goes only to the job log in logDir; system events also go to the default logger.
func Run(ctx context.Context, jobID, logDir string, spec *Spec, timeoutSec int32) error {
	if spec == nil {
		return errors.New("spec is nil")
	}
	if spec.Runtime == "" {
		return errors.New("runtime (Docker image) is required")
	}

	jobLogger, closeLog, err := openJobLog(logDir, jobID)
	if err != nil {
		return fmt.Errorf("job log: %w", err)
	}
	defer closeLog()

	dir, err := os.MkdirTemp("", "lute-job-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	logSystem(jobLogger, slog.LevelInfo, "temp dir", slog.String("path", dir))

	repo := strings.TrimSpace(spec.SourceRepository)
	if repo != "" {
		if err := validateGitHubRepo(repo); err != nil {
			return fmt.Errorf("invalid source_repository: %w", err)
		}
		if err := cloneRepo(ctx, dir, repo); err != nil {
			return fmt.Errorf("clone: %w", err)
		}
		logSystem(jobLogger, slog.LevelInfo, "clone ok")
	} else {
		logSystem(jobLogger, slog.LevelInfo, "no source repository; empty workspace")
	}

	scriptPath := filepath.Join(dir, commandScriptName)
	if err := os.WriteFile(scriptPath, []byte(spec.Command), 0700); err != nil {
		return fmt.Errorf("write command script: %w", err)
	}

	runCtx := ctx
	if timeoutSec > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("docker client: %w", err)
	}
	defer func() { _ = cli.Close() }()

	exitCode, err := runContainer(runCtx, cli, dir, spec, jobLogger)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("container exited with code %d", exitCode)
	}

	logSystem(jobLogger, slog.LevelInfo, "job finished successfully")
	return nil
}
