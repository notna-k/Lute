// Package agent keeps the worker connected to the core over gRPC and runs the jobs it is assigned.
package agent

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
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
	ServerAddr string
	// TLS dials core with TLS, as behind a proxy that terminates it; off is plaintext.
	TLS         bool
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
	// closeGrace bounds the wait for core to end the stream after the agent half-closed it.
	closeGrace = 30 * time.Second
)

type Agent struct {
	cfg    Config
	jobCtx context.Context
	jobs   sync.WaitGroup

	mu          sync.Mutex
	running     map[string]struct{} // job ids, reported again on every new stream
	pending     []*pb.JobResult     // results that found no stream; sent on the next one
	sess        *session
	draining    bool // takes no new jobs
	deleted     bool // core asked it to drain and stop
	drainedSent bool
	done        bool
	outcome     Outcome
	finishing   *Outcome // drained, but results wait for a stream before the agent stops
	stop        context.CancelFunc
}

// New returns an agent whose jobs run under jobCtx, which outlives any one connection.
func New(cfg Config, jobCtx context.Context) *Agent {
	return &Agent{cfg: cfg, jobCtx: jobCtx, running: map[string]struct{}{}}
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
		// Core ends the stream cleanly only after it has removed a drained worker; any
		// other end is a lost connection, and the next stream repeats the drain.
		if a.drainConfirmed(err) {
			return Deleted, nil
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
	return Stopped, false
}

func (a *Agent) drainConfirmed(streamErr error) bool {
	a.mu.Lock()
	sent := a.drainedSent
	a.drainedSent = false
	a.mu.Unlock()
	return sent && errors.Is(streamErr, io.EOF)
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
	s, stop := a.sess, a.stop
	waiting := s == nil && len(a.pending) > 0
	if waiting {
		a.finishing = &o
	} else {
		a.done, a.outcome = true, o
	}
	a.mu.Unlock()
	if waiting {
		slog.Info("Drained; reconnecting to report the last results before stopping")
		return
	}
	if s == nil || s.closeSend() != nil {
		stop()
		return
	}
	time.AfterFunc(closeGrace, stop)
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
	conn, err := Dial(a.cfg.ServerAddr, a.cfg.TLS)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+a.cfg.WorkerID+"."+a.cfg.Secret)
	stream, err := pb.NewWorkerServiceClient(conn).Connect(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	// Publishing the session and listing running jobs happen under one lock, and sends
	// wait for the registration: a job that finishes during the handover is either in
	// running_jobs and reports on this stream after it, or in pending and flushed here.
	s := &session{agent: a, stream: stream}
	s.sendMu.Lock()
	a.mu.Lock()
	running := make([]string, 0, len(a.running))
	for id := range a.running {
		running = append(running, id)
	}
	pending := a.pending
	a.pending = nil
	a.sess = s
	draining := a.draining && !a.deleted
	a.mu.Unlock()
	err = stream.Send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_Register{Register: &pb.WorkerRegistration{
		Queues:      a.cfg.Queues,
		Concurrency: a.cfg.Concurrency,
		Version:     a.cfg.Version,
		Protocol:    pb.Protocol,
		Engine:      a.cfg.Engine,
		RunningJobs: running,
	}}})
	for i := 0; err == nil && i < len(pending); i++ {
		err = stream.Send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_Result{Result: pending[i]}})
		if err == nil {
			pending[i] = nil
		}
	}
	s.sendMu.Unlock()
	if err != nil {
		a.mu.Lock()
		if a.sess == s {
			a.sess = nil
		}
		for _, r := range pending {
			if r != nil {
				a.pending = append(a.pending, r)
			}
		}
		a.mu.Unlock()
		return fmt.Errorf("send registration: %w", err)
	}

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
	a.flushPending()
	a.mu.Lock()
	var finishing *Outcome
	if len(a.pending) == 0 {
		finishing, a.finishing = a.finishing, nil
	}
	a.mu.Unlock()
	if finishing != nil {
		a.finishGracefully(*finishing)
	}

	slog.Info("Connected", "server", a.cfg.ServerAddr)
	return s.serve()
}

// Register enrols a new worker with a registration token, retrying while core is unreachable.
func Register(ctx context.Context, server string, useTLS bool, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	conn, err := Dial(server, useTLS)
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

// Dial connects to core. Without TLS the token, secret and job data cross the network in
// plaintext, so that is for a local core or a private network only.
func Dial(server string, useTLS bool) (*grpc.ClientConn, error) {
	creds := insecure.NewCredentials()
	if useTLS {
		creds = credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	}
	return grpc.NewClient(server, grpc.WithTransportCredentials(creds))
}
