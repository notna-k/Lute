package grpc

import (
	"testing"

	pb "github.com/lute/proto"
)

func TestAssignJobAfterShutdownIsRefused(t *testing.T) {
	wc := newWorkerConnection("w1", nil)
	wc.Concurrency = 2
	if !wc.AssignJob(&pb.JobAssignment{JobId: "before"}) {
		t.Fatal("assignment refused before the drain")
	}
	wc.Shutdown()
	if wc.AssignJob(&pb.JobAssignment{JobId: "after"}) {
		t.Fatal("assignment accepted after the drain began")
	}
	// Only the job queued before the drain is counted and waits to be sent ahead of the DrainSignal.
	if wc.ActiveJobs != 1 || len(wc.jobCh) != 1 {
		t.Errorf("active=%d queued=%d, want 1 and 1", wc.ActiveJobs, len(wc.jobCh))
	}
}

func TestAssignJobRespectsCapacity(t *testing.T) {
	wc := newWorkerConnection("w1", nil)
	if !wc.AssignJob(&pb.JobAssignment{JobId: "a"}) || wc.AssignJob(&pb.JobAssignment{JobId: "b"}) {
		t.Fatal("concurrency 1 should take exactly one job")
	}
	wc.release("a")
	if !wc.AssignJob(&pb.JobAssignment{JobId: "b"}) {
		t.Fatal("a released slot was not reused")
	}
}
