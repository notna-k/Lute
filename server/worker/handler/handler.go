package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/lute/worker/runner"
)

// HandlerFunc processes a job payload. Return nil on success, error on failure.
// timeoutSec is the job's timeout in seconds (0 means use default or no limit).
type HandlerFunc func(ctx context.Context, payload []byte, timeoutSec int32) error

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
func Execute(ctx context.Context, jobType string, payload []byte, timeoutSec int32) error {
	mu.RLock()
	fn, ok := handlers[jobType]
	mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler registered for job type %q", jobType)
	}

	log.Printf("Executing job type=%s payload_size=%d timeout_sec=%d", jobType, len(payload), timeoutSec)
	return fn(ctx, payload, timeoutSec)
}

func init() {
	Register("noop", func(_ context.Context, _ []byte, _ int32) error {
		log.Println("noop job executed")
		return nil
	})

	Register("container", containerHandler)
}

func containerHandler(ctx context.Context, payload []byte, timeoutSec int32) error {
	var spec runner.Spec
	if err := json.Unmarshal(payload, &spec); err != nil {
		return fmt.Errorf("decode container spec: %w", err)
	}
	_, err := runner.Run(ctx, &spec, timeoutSec)
	return err
}
