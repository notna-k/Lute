// Package agent keeps the worker connected to the core over gRPC and runs the jobs it is assigned.
package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/lute/proto"
)

// JobRunner executes assignments and serves their logs.
type JobRunner interface {
	Execute(ctx context.Context, a *pb.JobAssignment) error
	ReadLog(req *pb.JobLogRequest) *pb.JobLogResponse
}

type Config struct {
	ServerAddr  string
	WorkerID    string
	Secret      string
	Queues      []string
	Concurrency int32
	Version     string
	Engine      *pb.EngineInfo
	Jobs        JobRunner
}

// Outcome is why Run returned.
type Outcome int

const (
	Stopped Outcome = iota // its context ended
	Drained                // Drain finished: every job it had is done
	Deleted                // core deleted the worker; it should stop for good
)

const (
	maxBackoff = 30 * time.Second
	// deletedGrace bounds the wait for core to close the stream after the agent is done.
	deletedGrace = 30 * time.Second
)

type Agent struct {
	cfg    Config
	jobCtx context.Context
	jobs   sync.WaitGroup

	mu          sync.Mutex
	sess        *session
	draining    bool // takes no new jobs
	deleted     bool // core asked it to drain and stop
	drainedSent bool
	done        bool
	outcome     Outcome
	stop        context.CancelFunc
}

// New returns an agent whose jobs run under jobCtx, which outlives any one connection.
func New(cfg Config, jobCtx context.Context) *Agent {
	return &Agent{cfg: cfg, jobCtx: jobCtx}
}

// Run connects and reconnects with backoff until the agent is drained or deleted, ctx
// ends, or core refuses it. A refusal is returned as the gRPC status error.
func (a *Agent) Run(ctx context.Context) (Outcome, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	a.mu.Lock()
	a.stop = cancel
	a.mu.Unlock()

	backoff := time.Second
	for {
		err := a.connect(ctx)
		if outcome, ok := a.finished(); ok {
			return outcome, nil
		}
		if ctx.Err() != nil {
			return Stopped, nil
		}
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.NotFound:
				slog.Warn("worker was deleted; remove state.json to register again", "msg", st.Message())
				return Deleted, nil
			case codes.Unauthenticated, codes.PermissionDenied, codes.FailedPrecondition, codes.InvalidArgument:
				return Stopped, err
			}
		}

		slog.Warn("Stream disconnected, reconnecting", "err", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			if outcome, ok := a.finished(); ok {
				return outcome, nil
			}
			return Stopped, nil
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

// finished reports the outcome once drain or delete completed.
func (a *Agent) finished() (Outcome, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.done {
		return a.outcome, true
	}
	// Core closes the stream once it has forgotten a drained worker.
	if a.drainedSent {
		return Deleted, true
	}
	return Stopped, false
}

func (a *Agent) finish(o Outcome) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.done {
		return
	}
	a.done, a.outcome = true, o
	if a.stop != nil {
		a.stop()
	}
}

// Drain stops taking jobs and tells core; Run returns Drained once running jobs finish.
func (a *Agent) Drain() {
	a.mu.Lock()
	if a.draining && !a.deleted {
		a.mu.Unlock()
		return
	}
	a.draining = true
	a.mu.Unlock()
	slog.Info("Draining: no new jobs, waiting for running ones")
	_ = a.sendStatus(&pb.WorkerStatus{Draining: true})
	go func() {
		a.jobs.Wait()
		a.finishGracefully(Drained)
	}()
}

// finishGracefully half-closes the stream so core reads every result sent so far before it
// ends the stream; cancelling at once could drop the last one.
func (a *Agent) finishGracefully(o Outcome) {
	a.mu.Lock()
	if a.done {
		a.mu.Unlock()
		return
	}
	a.done, a.outcome = true, o
	s, stop := a.sess, a.stop
	a.mu.Unlock()
	if s == nil || s.closeSend() != nil {
		stop()
		return
	}
	time.AfterFunc(deletedGrace, stop)
}

// deleteRequested handles core's DrainSignal: finish running jobs, report drained, and
// let core close the stream once it has removed the worker.
func (a *Agent) deleteRequested() {
	a.mu.Lock()
	a.draining, a.deleted = true, true
	a.mu.Unlock()
	slog.Info("Worker deleted in the panel: finishing running jobs, then stopping")
	_ = a.sendStatus(&pb.WorkerStatus{Draining: true})
	go func() {
		a.jobs.Wait()
		if err := a.sendStatus(&pb.WorkerStatus{Drained: true}); err != nil {
			return // resent when core repeats the signal on the next stream
		}
		a.mu.Lock()
		a.drainedSent = true
		a.mu.Unlock()
		time.AfterFunc(deletedGrace, func() { a.finish(Deleted) })
	}()
}

func (a *Agent) sendStatus(st *pb.WorkerStatus) error {
	s := a.session()
	if s == nil {
		return errors.New("not connected")
	}
	err := s.send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_Status{Status: st}})
	if err != nil {
		slog.Warn("send status", "err", err)
	}
	return err
}

func (a *Agent) session() *session {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sess
}

// connect opens one authenticated stream, registers on it, and serves it until it breaks.
func (a *Agent) connect(ctx context.Context) error {
	conn, err := grpc.NewClient(a.cfg.ServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+a.cfg.WorkerID+"."+a.cfg.Secret)
	stream, err := pb.NewWorkerServiceClient(conn).Connect(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	s := &session{agent: a, stream: stream}
	if err := s.send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_Register{Register: &pb.WorkerRegistration{
		Queues:      a.cfg.Queues,
		Concurrency: a.cfg.Concurrency,
		Version:     a.cfg.Version,
		Protocol:    pb.Protocol,
		Engine:      a.cfg.Engine,
	}}}); err != nil {
		return fmt.Errorf("send registration: %w", err)
	}

	a.mu.Lock()
	a.sess = s
	draining := a.draining && !a.deleted
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		if a.sess == s {
			a.sess = nil
		}
		a.mu.Unlock()
	}()
	if draining {
		_ = s.send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_Status{Status: &pb.WorkerStatus{Draining: true}}})
	}

	slog.Info("Connected", "server", a.cfg.ServerAddr)
	return s.serve()
}

// Register enrols a new worker with a registration token, retrying while core is unreachable.
func Register(ctx context.Context, server string, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	conn, err := grpc.NewClient(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", server, err)
	}
	defer func() { _ = conn.Close() }()
	client := pb.NewWorkerServiceClient(conn)

	backoff := time.Second
	for {
		resp, err := client.Register(ctx, req)
		if err == nil {
			return resp, nil
		}
		if code := status.Code(err); code != codes.Unavailable && code != codes.DeadlineExceeded {
			return nil, err
		}
		slog.Warn("Core unreachable, retrying registration", "server", server, "err", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}
