package runs

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/db/types"
	"github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/queue"
	"github.com/lute/api/internal/testutil/pgtest"
	pb "github.com/lute/proto"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// fakeGateway stands in for the agents' side of gRPC; the database under it is real.
type fakeGateway struct {
	mu         sync.Mutex
	dispatched []string
	asked      []string // worker ids log requests went to
	logReq     *pb.JobLogRequest
	logResp    *pb.JobLogResponse
	logErr     error
}

func (g *fakeGateway) DispatchQueue(_ context.Context, q string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.dispatched = append(g.dispatched, q)
}

func (g *fakeGateway) RequestJobLog(_ context.Context, workerID string, req *pb.JobLogRequest) (*pb.JobLogResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.asked = append(g.asked, workerID)
	g.logReq = req
	if g.logErr != nil {
		return nil, g.logErr
	}
	if g.logResp == nil {
		return &pb.JobLogResponse{}, nil
	}
	return g.logResp, nil
}

type fixture struct {
	svc   *Service
	gw    *fakeGateway
	queue *queue.Engine
	execs *repos.JobExecutionRepository
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db, err := connection.Open(context.Background(), pgtest.NewDatabase(t))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f := &fixture{
		gw:    &fakeGateway{},
		queue: queue.NewEngine(db.DB, queue.Timings{}),
		execs: repos.NewJobExecutionRepository(db.DB),
	}
	f.svc = New(f.queue, queue.NewStats(db.DB), repos.NewRunRepository(db.DB), f.execs, f.gw)
	return f
}

func (f *fixture) enqueue(t *testing.T, run *models.Run) *models.Run {
	t.Helper()
	if run.Queue == "" {
		run.Queue = "build"
	}
	if run.Type == "" {
		run.Type = "container"
	}
	got, created, err := f.svc.Enqueue(context.Background(), run, Job{})
	if err != nil || !created {
		t.Fatalf("enqueue: created=%v err=%v", created, err)
	}
	return got
}

// runOn dispatches the job to workerID as the gRPC dispatcher would.
func (f *fixture) runOn(t *testing.T, jobID, workerID string) {
	t.Helper()
	ctx := context.Background()
	job, err := f.queue.Dequeue(ctx, "build", workerID)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if job == nil || job.ID != jobID {
		t.Fatalf("dequeued %+v, want %s", job, jobID)
	}
}

func TestEnqueueRecordsTheRunAndQueuesItsJob(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	user := id.New()

	run, _, err := f.svc.Enqueue(ctx, &models.Run{UserID: user, Queue: "build", Type: "container", JobSlug: "deploy"},
		Job{Payload: []byte(`{"command":"make"}`), Selector: map[string]string{"os": "linux"}})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if run.JobID == "" || run.ID.IsZero() {
		t.Fatalf("run was not stored with a job id: %+v", run)
	}

	job, err := f.svc.Job(ctx, run.JobID)
	if err != nil {
		t.Fatalf("the job is not on the queue: %v", err)
	}
	if job.Status != "pending" || job.Queue != "build" || job.Selector["os"] != "linux" {
		t.Errorf("queued job = %+v", job)
	}
	if job.Meta["run_id"] != run.ID.Hex() || job.Meta["user_id"] != user.Hex() || job.Meta["job_slug"] != "deploy" {
		t.Errorf("job meta does not point back at the run: %v", job.Meta)
	}
	if len(f.gw.dispatched) != 1 || f.gw.dispatched[0] != "build" {
		t.Errorf("dispatched = %v, want [build]", f.gw.dispatched)
	}

	owned, err := f.svc.Owned(ctx, user, run.JobID)
	if err != nil || owned.ID != run.ID {
		t.Fatalf("owner cannot load the run: %v", err)
	}
	if _, err := f.svc.Owned(ctx, id.New(), run.JobID); !errors.Is(err, repos.ErrNotFound) {
		t.Errorf("another user loads the run: err = %v, want ErrNotFound", err)
	}
}

func TestEnqueueWithAUsedIdempotencyKeyReturnsTheFirstRun(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	user := id.New()

	first := f.enqueue(t, &models.Run{UserID: user, IdempotencyKey: "deploy-1"})
	again, created, err := f.svc.Enqueue(ctx, &models.Run{UserID: user, Queue: "build", Type: "container", IdempotencyKey: "deploy-1"}, Job{})
	if err != nil {
		t.Fatalf("enqueue again: %v", err)
	}
	if created || again.JobID != first.JobID {
		t.Fatalf("created=%v job=%s, want the first run %s", created, again.JobID, first.JobID)
	}
	if len(f.gw.dispatched) != 1 {
		t.Errorf("the repeat was dispatched too: %v", f.gw.dispatched)
	}

	// Keys are per user.
	other := f.enqueue(t, &models.Run{UserID: id.New(), IdempotencyKey: "deploy-1"})
	if other.JobID == first.JobID {
		t.Error("another user's key collided with the first user's run")
	}
}

func TestRetryStartsTheJobOver(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	run := f.enqueue(t, &models.Run{UserID: id.New()})
	f.runOn(t, run.JobID, "worker-a")
	if err := f.queue.Fail(ctx, run.JobID, "boom"); err != nil {
		t.Fatalf("fail: %v", err)
	}

	job, err := f.svc.Retry(ctx, run.JobID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	stored, err := f.svc.Job(ctx, run.JobID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, j := range []*queue.Job{job, stored} {
		if j.Status != "pending" || j.Attempts != 0 || j.Error != "" || j.WorkerID != "" {
			t.Errorf("retried job still carries its last attempt: %+v", j)
		}
	}
	if len(f.gw.dispatched) != 2 {
		t.Errorf("the retry was not dispatched: %v", f.gw.dispatched)
	}

	if _, err := f.svc.Retry(ctx, "no-such-job"); !errors.Is(err, repos.ErrNotFound) {
		t.Errorf("retry unknown job: err = %v, want ErrNotFound", err)
	}
}

func TestCancelOnlyStopsPendingJobs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	pending := f.enqueue(t, &models.Run{UserID: id.New()})
	if err := f.svc.Cancel(ctx, pending.JobID); err != nil {
		t.Fatalf("cancel a pending job: %v", err)
	}

	running := f.enqueue(t, &models.Run{UserID: id.New()})
	f.runOn(t, running.JobID, "worker-a")
	if err := f.svc.Cancel(ctx, running.JobID); !errors.Is(err, queue.ErrNotCancellable) {
		t.Errorf("cancel a running job: err = %v, want ErrNotCancellable", err)
	}
	if err := f.svc.Cancel(ctx, "no-such-job"); !errors.Is(err, repos.ErrNotFound) {
		t.Errorf("cancel unknown job: err = %v, want ErrNotFound", err)
	}
}

func TestLogsAskTheWorkerHoldingTheLog(t *testing.T) {
	ctx := context.Background()
	q := LogQuery{Direction: "tail", Limit: 10}

	t.Run("a running job asks the worker running it", func(t *testing.T) {
		f := newFixture(t)
		run := f.enqueue(t, &models.Run{UserID: id.New()})
		f.runOn(t, run.JobID, "worker-a")
		// A stale record from an earlier attempt must not win over the live one.
		if err := f.execs.Upsert(ctx, &models.JobExecution{JobID: run.JobID, WorkerID: "worker-old"}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if _, err := f.svc.Logs(ctx, run.JobID, q); err != nil {
			t.Fatalf("logs: %v", err)
		}
		if f.gw.asked[0] != "worker-a" {
			t.Errorf("asked %v, want worker-a", f.gw.asked)
		}
	})

	t.Run("a finished job asks the worker in its execution record", func(t *testing.T) {
		f := newFixture(t)
		run := f.enqueue(t, &models.Run{UserID: id.New()})
		f.runOn(t, run.JobID, "worker-a")
		if err := f.queue.Complete(ctx, run.JobID, 10); err != nil {
			t.Fatalf("complete: %v", err)
		}
		if err := f.execs.Upsert(ctx, &models.JobExecution{JobID: run.JobID, WorkerID: "worker-b", Success: true}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if _, err := f.svc.Logs(ctx, run.JobID, q); err != nil {
			t.Fatalf("logs: %v", err)
		}
		if f.gw.asked[0] != "worker-b" {
			t.Errorf("asked %v, want worker-b", f.gw.asked)
		}
	})

	t.Run("a just-finished job without a record falls back to its dispatch host", func(t *testing.T) {
		f := newFixture(t)
		run := f.enqueue(t, &models.Run{UserID: id.New()})
		f.runOn(t, run.JobID, "worker-a")
		if err := f.queue.Complete(ctx, run.JobID, 10); err != nil {
			t.Fatalf("complete: %v", err)
		}
		if _, err := f.svc.Logs(ctx, run.JobID, q); err != nil {
			t.Fatalf("logs: %v", err)
		}
		if f.gw.asked[0] != "worker-a" {
			t.Errorf("asked %v, want worker-a", f.gw.asked)
		}
	})

	t.Run("a job whose queue slot is gone still reads from its record", func(t *testing.T) {
		f := newFixture(t)
		if err := f.execs.Upsert(ctx, &models.JobExecution{JobID: "purged", WorkerID: "worker-c"}); err != nil {
			t.Fatalf("upsert: %v", err)
		}
		if _, err := f.svc.Logs(ctx, "purged", q); err != nil {
			t.Fatalf("logs: %v", err)
		}
		if f.gw.asked[0] != "worker-c" {
			t.Errorf("asked %v, want worker-c", f.gw.asked)
		}
	})

	t.Run("a job no worker picked up has no logs", func(t *testing.T) {
		f := newFixture(t)
		run := f.enqueue(t, &models.Run{UserID: id.New()})
		if _, err := f.svc.Logs(ctx, run.JobID, q); !errors.Is(err, ErrNoLogs) {
			t.Errorf("err = %v, want ErrNoLogs", err)
		}
		if len(f.gw.asked) != 0 {
			t.Errorf("a worker was asked anyway: %v", f.gw.asked)
		}
	})

	t.Run("an unknown job is not found", func(t *testing.T) {
		f := newFixture(t)
		if _, err := f.svc.Logs(ctx, "no-such-job", q); !errors.Is(err, repos.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestLogsPassThePageThroughAndMapGatewayErrors(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	run := f.enqueue(t, &models.Run{UserID: id.New()})
	f.runOn(t, run.JobID, "worker-a")

	f.gw.logResp = &pb.JobLogResponse{Lines: []string{"a", "b"}, HasMore: true, NextAnchor: 42, FileSize: 99}
	page, err := f.svc.Logs(ctx, run.JobID, LogQuery{Direction: "head", Limit: 2, Cursor: 7})
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if req := f.gw.logReq; req.GetDirection() != pb.LogReadDirection_LOG_READ_HEAD || req.GetLimit() != 2 || req.GetAnchorOffset() != 7 || req.GetJobId() != run.JobID {
		t.Errorf("request = %v", req)
	}
	if len(page.Lines) != 2 || !page.HasMore || page.NextCursor != "42" || page.FileSize != 99 || page.Direction != "head" {
		t.Errorf("page = %+v", page)
	}

	f.gw.logResp = &pb.JobLogResponse{}
	page, err = f.svc.Logs(ctx, run.JobID, LogQuery{Direction: "tail", Limit: 2})
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if page.Lines == nil || page.NextCursor != "" {
		t.Errorf("an empty last page = %+v, want [] lines and no cursor", page)
	}

	f.gw.logErr = grpc.ErrNoConnection
	if _, err := f.svc.Logs(ctx, run.JobID, LogQuery{Direction: "tail", Limit: 2}); !errors.Is(err, ErrWorkerOffline) {
		t.Errorf("worker gone: err = %v, want ErrWorkerOffline", err)
	}
	f.gw.logErr = errors.New("disk on fire")
	if _, err := f.svc.Logs(ctx, run.JobID, LogQuery{Direction: "tail", Limit: 2}); !errors.Is(err, ErrLogRead) {
		t.Errorf("worker failed: err = %v, want ErrLogRead", err)
	}
}

func TestParseLogQuery(t *testing.T) {
	cases := []struct {
		direction, limit, cursor string
		want                     LogQuery
		wantErr                  bool
	}{
		{"", "", "", LogQuery{Direction: "tail", Limit: defaultLogLimit}, false},
		{"head", "50", "12", LogQuery{Direction: "head", Limit: 50, Cursor: 12}, false},
		{"tail", "100000", "", LogQuery{Direction: "tail", Limit: maxLogLimit}, false},
		{"sideways", "", "", LogQuery{}, true},
		{"tail", "0", "", LogQuery{}, true},
		{"tail", "-1", "", LogQuery{}, true},
		{"tail", "ten", "", LogQuery{}, true},
		{"tail", "", "later", LogQuery{}, true},
	}
	for _, tc := range cases {
		got, err := ParseLogQuery(tc.direction, tc.limit, tc.cursor)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseLogQuery(%q, %q, %q): err = %v, wantErr %v", tc.direction, tc.limit, tc.cursor, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("ParseLogQuery(%q, %q, %q) = %+v, want %+v", tc.direction, tc.limit, tc.cursor, got, tc.want)
		}
	}
}

func TestHistoryGroupsRunsBySlugWithTheirExecutions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	user := id.New()

	// Backdated so the order does not hang on two inserts landing in one millisecond.
	earlier := &models.Run{UserID: user, JobSlug: "deploy"}
	earlier.CreatedAt = types.NewMilliTime(time.Now().Add(-time.Minute))
	older := f.enqueue(t, earlier)
	newer := f.enqueue(t, &models.Run{UserID: user, JobSlug: "deploy"})
	f.enqueue(t, &models.Run{UserID: user, JobSlug: "lint"})
	f.enqueue(t, &models.Run{UserID: id.New(), JobSlug: "deploy"})
	if err := f.execs.Upsert(ctx, &models.JobExecution{JobID: older.JobID, Success: true}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	bySlug, execs, err := f.svc.History(ctx, user, []string{"deploy", "lint"}, 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	deploy := bySlug["deploy"]
	if len(deploy) != 2 || deploy[0].JobID != newer.JobID || deploy[1].JobID != older.JobID {
		t.Fatalf("deploy history = %v, want newest first and only this user's", deploy)
	}
	if len(bySlug["lint"]) != 1 {
		t.Errorf("lint history = %v", bySlug["lint"])
	}
	if len(execs) != 1 || execs[older.JobID] == nil || !execs[older.JobID].Success {
		t.Errorf("execs = %v, want only the finished run", execs)
	}
}
