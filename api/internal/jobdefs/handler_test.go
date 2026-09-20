package jobdefs

import (
	"strconv"
	"testing"

	"github.com/lute/api/internal/db/models"
)

// runsFor builds a newest-first slice of runs with ids "0", "1", ... matching
// the order the repository returns them in.
func runsFor(n int) []models.Run {
	runs := make([]models.Run, n)
	for i := range runs {
		runs[i].JobID = strconv.Itoa(i)
	}
	return runs
}

func execs(successByJobID map[string]bool) map[string]*models.JobExecution {
	out := map[string]*models.JobExecution{}
	for id, ok := range successByJobID {
		out[id] = &models.JobExecution{JobID: id, Success: ok}
	}
	return out
}

func TestRecentStatusesReadsOldestFirst(t *testing.T) {
	// Newest-first runs: 0 (newest) … 3 (oldest).
	got := recentStatuses(runsFor(4), execs(map[string]bool{
		"1": false,
		"2": true,
		"3": true,
	}), "running")

	want := []string{"passed", "passed", "failed", "running"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestRecentStatusesUsesPassedLastStatusForNewest(t *testing.T) {
	// The newest build's status is resolved against the queue by the caller, so
	// it must win over whatever the execution record says.
	got := recentStatuses(runsFor(1), execs(map[string]bool{"0": false}), "passed")
	if len(got) != 1 || got[0] != "passed" {
		t.Fatalf("got %v, want [passed]", got)
	}
}

func TestRecentStatusesTreatsMissingExecutionAsQueued(t *testing.T) {
	got := recentStatuses(runsFor(2), execs(nil), "queued")
	want := []string{"queued", "queued"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestRecentStatusesCapsAtRecentWindow(t *testing.T) {
	got := recentStatuses(runsFor(recentWindow+10), execs(nil), "passed")
	if len(got) != recentWindow {
		t.Fatalf("got %d entries, want %d", len(got), recentWindow)
	}
	// The window keeps the newest builds, which land at the end of the strip.
	if got[len(got)-1] != "passed" {
		t.Fatalf("newest entry = %q, want passed", got[len(got)-1])
	}
}

func TestRecentStatusesEmptyForNoRuns(t *testing.T) {
	if got := recentStatuses(nil, execs(nil), "passed"); len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}
