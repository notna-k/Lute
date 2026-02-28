package handler

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// HandlerFunc processes a job payload. Return nil on success, error on failure.
type HandlerFunc func(ctx context.Context, payload []byte) error

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
func Execute(ctx context.Context, jobType string, payload []byte) error {
	mu.RLock()
	fn, ok := handlers[jobType]
	mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler registered for job type %q", jobType)
	}

	log.Printf("Executing job type=%s payload_size=%d", jobType, len(payload))
	return fn(ctx, payload)
}

func init() {
	Register("noop", func(_ context.Context, _ []byte) error {
		log.Println("noop job executed")
		return nil
	})
}
