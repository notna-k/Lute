// Package runner runs "container" jobs as sibling containers on the agent's engine. Each
// job gets its own network and workspace volume, so no host path is shared with the agent.
package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/docker/docker/client"

	"github.com/lute/worker/internal/datadir"
	"github.com/lute/worker/internal/joblog"
)

// Spec is the JSON payload of a "container" job (mirrors proto ContainerJobSpec).
type Spec struct {
	SourceRepository string            `json:"source_repository"`
	Runtime          string            `json:"runtime"`
	RequestParams    map[string]string `json:"request_params"`
	Command          string            `json:"command"`
}

// Labels on every container, volume and network a job creates, so the reaper finds them.
const (
	LabelWorker = "lute.worker"
	LabelJob    = "lute.job"
)

const (
	workspaceMount = "/workspace"
	scriptPath     = "/lute/cmd.sh"
	cleanupTimeout = 30 * time.Second
)

// Limits apply to every job container; zero leaves a resource unlimited.
type Limits struct {
	Memory   int64
	NanoCPUs int64
	Pids     int64
}

type Runner struct {
	Docker   client.APIClient
	Data     *datadir.Dir
	WorkerID string
	GitImage string
	Limits   Limits
}

// Run executes one job. Its output goes to jobs/<id>/log, and jobs/<id>/meta.json records
// what ran and how it ended.
func (r *Runner) Run(ctx context.Context, jobID string, spec *Spec, timeoutSec int32) (err error) {
	if spec == nil {
		return errors.New("spec is nil")
	}
	if spec.Runtime == "" {
		return errors.New("runtime (Docker image) is required")
	}
	repo := strings.TrimSpace(spec.SourceRepository)
	if err := validateGitHubRepo(repo); err != nil {
		return fmt.Errorf("invalid source_repository: %w", err)
	}

	if _, err := r.Data.JobDir(jobID); err != nil {
		return fmt.Errorf("job dir: %w", err)
	}
	logPath, err := joblog.Path(r.Data.Root, jobID)
	if err != nil {
		return err
	}
	jobLogger, closeLog, err := openJobLog(logPath)
	if err != nil {
		return fmt.Errorf("job log: %w", err)
	}
	defer closeLog()

	meta := &datadir.Meta{
		JobID:      jobID,
		WorkerID:   r.WorkerID,
		Image:      spec.Runtime,
		Repository: repo,
		StartedAt:  time.Now().UTC(),
	}
	r.writeMeta(meta)
	defer func() {
		finished := time.Now().UTC()
		meta.FinishedAt = &finished
		if err != nil {
			meta.Error = err.Error()
		}
		r.writeMeta(meta)
	}()

	runCtx := ctx
	if timeoutSec > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}

	j := &job{Runner: r, id: jobID, log: jobLogger}
	defer j.cleanup()
	if err := j.prepare(runCtx); err != nil {
		return err
	}
	if repo != "" {
		commit, err := j.clone(runCtx, repo)
		if err != nil {
			return fmt.Errorf("clone: %w", err)
		}
		meta.Commit = commit
	} else {
		logSystem(jobLogger, slog.LevelInfo, "no source repository; empty workspace")
	}

	digest, err := j.pull(runCtx, spec.Runtime, true)
	if err != nil {
		return err
	}
	meta.ImageDigest = digest

	exitCode, err := j.runCommand(runCtx, spec)
	if err != nil {
		return err
	}
	meta.ExitCode = &exitCode
	if exitCode != 0 {
		return fmt.Errorf("container exited with code %d", exitCode)
	}
	logSystem(jobLogger, slog.LevelInfo, "job finished successfully")
	return nil
}

func (r *Runner) writeMeta(m *datadir.Meta) {
	if err := r.Data.WriteMeta(m); err != nil {
		slog.Warn("write job meta", "job_id", m.JobID, "err", err)
	}
}

// LogFile is the job's log path relative to the data dir, as reported to core.
func LogFile(jobID string) string { return joblog.FileName(jobID) }
