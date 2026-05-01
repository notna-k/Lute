package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/lute/worker/internal/runner"
)

// HandlerFunc processes a job payload. Return nil on success, error on failure.
// timeoutSec is the job's timeout in seconds (0 means use default or no limit).
type HandlerFunc func(ctx context.Context, jobID string, payload []byte, timeoutSec int32) error

// JobLogsDir is the base directory for persisting job log files (job-{id}.log).
// When empty, runner job logs are discarded (JSON slog to io.Discard).
var JobLogsDir string

var (
	mu       sync.RWMutex
	handlers = make(map[string]HandlerFunc)
)

// Register adds a handler for the given job type.
func Register(jobType string, fn HandlerFunc) {
	mu.Lock()
	defer mu.Unlock()
	handlers[jobType] = fn
}

// Execute runs the handler registered for the given job type.
// Returns an error if no handler is registered or the handler fails.
func Execute(ctx context.Context, jobID, jobType string, payload []byte, timeoutSec int32) error {
	mu.RLock()
	fn, ok := handlers[jobType]
	mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler registered for job type %q", jobType)
	}

	slog.Info("Executing job", "type", jobType, "payload_size", len(payload), "timeout_sec", timeoutSec)
	return fn(ctx, jobID, payload, timeoutSec)
}

func init() {
	Register("noop", func(_ context.Context, _ string, _ []byte, _ int32) error {
		slog.Info("noop job executed")
		return nil
	})

	Register("container", containerHandler)
}

func containerHandler(ctx context.Context, jobID string, payload []byte, timeoutSec int32) error {
	var spec runner.Spec
	if err := json.Unmarshal(payload, &spec); err != nil {
		return fmt.Errorf("decode container spec: %w", err)
	}
	_, err := runner.Run(ctx, jobID, JobLogsDir, &spec, timeoutSec)
	return err
}
