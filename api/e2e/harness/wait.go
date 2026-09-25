//go:build e2e

package harness

import (
	"testing"
	"time"
)

const pollInterval = 25 * time.Millisecond

// WaitFor fails the test with `what` if cond is not true by the deadline. Wait on state, never sleep.
func WaitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("timed out after %s waiting for %s", timeout, what)
}

func Eventually[T any](t *testing.T, timeout time.Duration, what string, f func() (T, bool)) T {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last T
	for time.Now().Before(deadline) {
		v, ok := f()
		if ok {
			return v
		}
		last = v
		time.Sleep(pollInterval)
	}
	t.Fatalf("timed out after %s waiting for %s (last value: %+v)", timeout, what, last)
	return last
}

// Never fails if cond becomes true within the window, for asserting that something does not happen.
func Never(t *testing.T, window time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		if cond() {
			t.Fatalf("%s happened, and it should not have", what)
		}
		time.Sleep(pollInterval)
	}
}
