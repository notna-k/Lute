package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSplitList(t *testing.T) {
	got := splitList(" default, gpu ,,linux ")
	if diff := cmp.Diff([]string{"default", "gpu", "linux"}, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestShowLogMissingFile(t *testing.T) {
	if err := showLog(filepath.Join(t.TempDir(), "nope.log"), 10, false); err == nil {
		t.Error("want an error for a missing log file")
	}
}

func TestShowLogWithoutFollowReturns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.log")
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{0, 1, 10} {
		if err := showLog(path, n, false); err != nil {
			t.Errorf("showLog(n=%d): %v", n, err)
		}
	}
}

func TestFollowStopsOnSignal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.log")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stop := make(chan os.Signal, 1)
	stop <- os.Interrupt
	if err := followFrom(path, 0, stop); err != nil {
		t.Fatal(err)
	}
}
