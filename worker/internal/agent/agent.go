// Package agent keeps the worker connected to the core over gRPC and runs the jobs it is assigned.
package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/lute/proto"
)

// Config describes one worker agent.
type Config struct {
	ServerAddr  string
	WorkerID    string
	Queues      []string
	Concurrency int32
	// JobLogsDir holds per-job log files; empty discards job logs.
	JobLogsDir string
}

const maxBackoff = 30 * time.Second

// errShutdown means the core asked this process to exit; it is not retried.
var errShutdown = errors.New("server requested worker shutdown")

// Run connects to the core and reconnects with backoff until ctx ends or the core
// tells the worker to stop for good.
func Run(ctx context.Context, cfg Config) {
	backoff := time.Second
	for ctx.Err() == nil {
		err := connect(ctx, cfg)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errShutdown) {
			slog.Info("Worker shutdown acknowledged, exiting")
			return
		}
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.NotFound:
				slog.Info("Server reports this worker no longer exists; exiting", "msg", st.Message())
				return
			case codes.FailedPrecondition:
				slog.Info("Server rejected connection with a non-retryable condition; exiting", "msg", st.Message())
				return
			}
		}

		slog.Warn("Stream disconnected, reconnecting", "err", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

// connect opens one stream, registers, and serves it until it breaks.
func connect(ctx context.Context, cfg Config) error {
	conn, err := grpc.NewClient(cfg.ServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	stream, err := pb.NewWorkerServiceClient(conn).Connect(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	s := &session{cfg: cfg, stream: stream}
	// The core reads only the worker id from the first message and ignores its payload.
	if err := s.send(&pb.WorkerMessage{}); err != nil {
		return fmt.Errorf("send initial: %w", err)
	}
	if err := s.send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_Register{
		Register: &pb.WorkerRegistration{Queues: cfg.Queues, Concurrency: cfg.Concurrency},
	}}); err != nil {
		return fmt.Errorf("send registration: %w", err)
	}

	slog.Info("Connected", "server", cfg.ServerAddr)
	return s.serve(ctx)
}
