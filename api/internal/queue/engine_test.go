package queue

import (
	"context"
	"errors"
	"os"
	"testing"

	"gorm.io/gorm"

	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

func newTestEngine(t *testing.T) (*Engine, *gorm.DB) {
	t.Helper()
	db, err := connection.Open(context.Background(), pgtest.NewDatabase(t))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewEngine(db.DB, Timings{}), db.DB
}

const testWorker = "worker-7"

// dispatch enqueues a job and leases it to testWorker, exactly as the dispatcher would.
func dispatch(t *testing.T, r *Engine, jobID string, opts EnqueueOpts) *Job {
	t.Helper()
	ctx := context.Background()
	if err := r.Enqueue(ctx, &Job{ID: jobID, Queue: "build", Type: "container"}, opts); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	job, err := r.Dequeue(ctx, "build", testWorker)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if job == nil {
		t.Fatal("dequeue returned no job")
	}
	return job
}

func slotOf(t *testing.T, db *gorm.DB, jobID string) models.QueueSlot {
	t.Helper()
	var slot models.QueueSlot
	if err := db.Where("job_id = ?", jobID).First(&slot).Error; err != nil {
		t.Fatalf("load slot %s: %v", jobID, err)
	}
	return slot
}

// expireLease backdates a slot's lease so the next sweep treats it as lost.
func expireLease(t *testing.T, db *gorm.DB, jobID string) {
	t.Helper()
	if err := db.Model(&models.QueueSlot{}).Where("job_id = ?", jobID).
		Update("lease_expires_at_ms", int64(1)).Error; err != nil {
		t.Fatalf("expire lease %s: %v", jobID, err)
	}
}

func TestDequeueLeasesTheJobAndCompleteReleasesIt(t *testing.T) {
	r, db := newTestEngine(t)
	ctx := context.Background()

	dispatch(t, r, "job-1", EnqueueOpts{TimeoutSec: 30})

	leased := slotOf(t, db, "job-1")
	if leased.LeaseExpiresAtMS <= nowMilli() {
		t.Fatalf("dequeue left no live lease: lease_expires_at_ms = %d", leased.LeaseExpiresAtMS)
	}

	if err := r.Complete(ctx, "job-1", 1234); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if got := slotOf(t, db, "job-1").LeaseExpiresAtMS; got != 0 {
		t.Fatalf("complete left the lease behind: lease_expires_at_ms = %d", got)
	}

	// The reported runtime is kept on the job: it is what a build shows as its duration
	// in the window before the execution record is written.
	done, err := r.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if done.ElapsedMs != 1234 {
		t.Errorf("elapsed_ms = %d, want the 1234 reported to Complete", done.ElapsedMs)
	}

	// A finished job must never look reapable, or the sweep would fail a build that passed.
	claimed, err := r.ClaimExpiredLeases(ctx)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("claimed a completed job: %+v", claimed)
	}
}

func TestClaimExpiredLeasesReportsTheLostJob(t *testing.T) {
	r, db := newTestEngine(t)
	ctx := context.Background()

	dispatch(t, r, "job-1", EnqueueOpts{TimeoutSec: 30})
	expireLease(t, db, "job-1")

	claimed, err := r.ClaimExpiredLeases(ctx)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("got %d claims, want 1: %+v", len(claimed), claimed)
	}
	if claimed[0].JobID != "job-1" || claimed[0].Queue != "build" || claimed[0].WorkerID != testWorker {
		t.Fatalf("claim lost the job's identity: %+v", claimed[0])
	}
}

func TestClaimExpiredLeasesIsExclusive(t *testing.T) {
	r, db := newTestEngine(t)
	ctx := context.Background()

	dispatch(t, r, "job-1", EnqueueOpts{TimeoutSec: 30})
	expireLease(t, db, "job-1")

	first, err := r.ClaimExpiredLeases(ctx)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first claim got %d, want 1", len(first))
	}

	// A second sweeper must not take a job that is already being reaped.
	second, err := r.ClaimExpiredLeases(ctx)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("second claim took the same job: %+v", second)
	}
}

func TestReapedJobIsRetriedAndStopsLookingRunning(t *testing.T) {
	r, db := newTestEngine(t)
	ctx := context.Background()

	dispatch(t, r, "job-1", EnqueueOpts{TimeoutSec: 30, MaxRetries: 3})
	expireLease(t, db, "job-1")

	if _, err := r.ClaimExpiredLeases(ctx); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := r.Fail(ctx, "job-1", "worker stopped reporting"); err != nil {
		t.Fatalf("fail: %v", err)
	}

	slot := slotOf(t, db, "job-1")
	if slot.Lane != enums.QueueLaneDelayed {
		t.Fatalf("reaped job did not go back for a retry: lane = %q", slot.Lane)
	}
	if slot.LeaseExpiresAtMS != 0 {
		t.Fatalf("retry kept a lease: lease_expires_at_ms = %d", slot.LeaseExpiresAtMS)
	}

	job, err := r.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Status != enums.QueueJobPending {
		t.Fatalf("panel would still show this build as %q", job.Status)
	}
}

func TestReapedJobWithNoRetriesLeftGoesToTheDLQ(t *testing.T) {
	r, db := newTestEngine(t)
	ctx := context.Background()

	dispatch(t, r, "job-1", EnqueueOpts{TimeoutSec: 30, MaxRetries: 1})
	expireLease(t, db, "job-1")

	if _, err := r.ClaimExpiredLeases(ctx); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := r.Fail(ctx, "job-1", "worker stopped reporting"); err != nil {
		t.Fatalf("fail: %v", err)
	}

	job, err := r.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Status != enums.QueueJobDead {
		t.Fatalf("status = %q, want dead", job.Status)
	}
	if got := slotOf(t, db, "job-1").LeaseExpiresAtMS; got != 0 {
		t.Fatalf("dead job kept a lease: lease_expires_at_ms = %d", got)
	}

	ids, err := r.DLQList(ctx, "build", 0, 10)
	if err != nil {
		t.Fatalf("dlq list: %v", err)
	}
	if len(ids) != 1 || ids[0] != "job-1" {
		t.Fatalf("dlq = %v, want [job-1]", ids)
	}
}

func TestLateResultForAReapedJobIsRejected(t *testing.T) {
	r, db := newTestEngine(t)
	ctx := context.Background()

	dispatch(t, r, "job-1", EnqueueOpts{TimeoutSec: 30, MaxRetries: 3})
	expireLease(t, db, "job-1")
	if _, err := r.ClaimExpiredLeases(ctx); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := r.Fail(ctx, "job-1", "worker stopped reporting"); err != nil {
		t.Fatalf("fail: %v", err)
	}

	// The worker finally reports, but the job is already queued for another attempt.
	if err := r.Complete(ctx, "job-1", 500); !errors.Is(err, ErrJobNotRunning) {
		t.Fatalf("complete after reap returned %v, want ErrJobNotRunning", err)
	}
	if err := r.Fail(ctx, "job-1", "late failure"); !errors.Is(err, ErrJobNotRunning) {
		t.Fatalf("fail after reap returned %v, want ErrJobNotRunning", err)
	}

	if lane := slotOf(t, db, "job-1").Lane; lane != enums.QueueLaneDelayed {
		t.Fatalf("late result disturbed the retry: lane = %q", lane)
	}
}

func TestLateSuccessCannotRevivADeadLetteredJob(t *testing.T) {
	r, db := newTestEngine(t)
	ctx := context.Background()

	// Out of retries: the reap dead-letters the job but leaves it in the dispatched lane.
	dispatch(t, r, "job-1", EnqueueOpts{TimeoutSec: 30, MaxRetries: 1})
	expireLease(t, db, "job-1")
	if _, err := r.ClaimExpiredLeases(ctx); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := r.Fail(ctx, "job-1", "worker stopped reporting"); err != nil {
		t.Fatalf("fail: %v", err)
	}

	if err := r.Complete(ctx, "job-1", 500); !errors.Is(err, ErrJobNotRunning) {
		t.Fatalf("complete after dead-lettering returned %v, want ErrJobNotRunning", err)
	}

	job, err := r.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if job.Status != enums.QueueJobDead {
		t.Fatalf("a late success reported the build as %q; it is dead", job.Status)
	}
}

func TestRequeuedJobClearsItsOldLease(t *testing.T) {
	r, db := newTestEngine(t)
	ctx := context.Background()

	dispatch(t, r, "job-1", EnqueueOpts{TimeoutSec: 30, MaxRetries: 1})
	expireLease(t, db, "job-1")
	if _, err := r.ClaimExpiredLeases(ctx); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := r.Fail(ctx, "job-1", "worker stopped reporting"); err != nil {
		t.Fatalf("fail: %v", err)
	}

	// A DLQ retry re-uses the slot row; a stale lease would make the sweep reap a queued job.
	n, err := r.DLQRetryAll(ctx, "build")
	if err != nil {
		t.Fatalf("dlq retry all: %v", err)
	}
	if n != 1 {
		t.Fatalf("retried %d jobs, want 1", n)
	}

	slot := slotOf(t, db, "job-1")
	if slot.Lane != enums.QueueLaneReady {
		t.Fatalf("lane = %q, want ready", slot.Lane)
	}
	if slot.LeaseExpiresAtMS != 0 {
		t.Fatalf("requeue kept the old lease: lease_expires_at_ms = %d", slot.LeaseExpiresAtMS)
	}
}
