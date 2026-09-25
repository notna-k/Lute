//go:build e2e

package harness

import (
	"testing"
	"time"
)

// pollInterval is how often a wait re-checks. Short enough that a test does not sit
// idle, long enough that polling does not dominate the stack's CPU.
const pollInterval = 25 * time.Millisecond

// WaitFor blocks until cond is true, failing the test with what it was waiting for
// if the deadline passes. Tests wait on observable state, never on a sleep: a sleep
// either makes the suite slow or makes it flaky, usually both.
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

// Eventually blocks until f reports a usable value and returns it.
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

// Never fails if cond becomes true within the window. Use it for the absence of an
// action — a build that must not be dispatched, an event that must not arrive.
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
