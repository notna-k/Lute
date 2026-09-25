// Package server runs core: the HTTP API, the gRPC endpoint agents connect to, and
// the background loops (queue sweep, heartbeats, snapshots, webhooks).
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/queue"
	"github.com/lute/api/internal/router"
	"github.com/lute/api/internal/setup"
	"github.com/lute/api/internal/webhooks"
	"github.com/lute/api/internal/websocket"
	"github.com/lute/api/internal/worker"
)

type Server struct {
	http         *http.Server
	httpListener net.Listener
	grpc         *grpc.Server
	loops        []func(context.Context)

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(d *setup.Deps) *Server {
	cfg := d.Config
	hub := websocket.NewHub()

	grpcServer := grpc.NewServer(cfg, d.Workers, d.JobExecutions, d.Queue, d.Stats, hub)
	grpcServer.WebhookEmitter = webhooks.NewEmitter(d.Runs, d.Webhooks)

	heartbeat := worker.NewHeartbeatChecker(d.Workers, grpcServer.ConnMgr,
		cfg.Heartbeat.CheckInterval, cfg.Heartbeat.PingTimeout, cfg.Heartbeat.MaxRetries)
	grpcServer.OnConnectionRegistered = heartbeat.TriggerCheck

	scheduler := queue.NewScheduler(d.Queue, cfg.Queue.PollInterval,
		func(ctx context.Context, queueNames []string) {
			for _, q := range queueNames {
				grpcServer.DispatchQueue(ctx, q)
			}
		},
		grpcServer.HandleExpiredLeases,
	)

	return &Server{
		http: &http.Server{
			Addr:         cfg.Server.Host + ":" + cfg.Server.Port,
			Handler:      router.New(d, hub, grpcServer),
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
		},
		grpc: grpcServer,
		loops: []func(context.Context){
			hub.Run,
			heartbeat.Run,
			scheduler.Run,
			worker.NewWorkerSnapshotJob(d.Workers, d.WorkerSnapshots, cfg.Metrics.SnapshotInterval).Run,
			webhooks.NewDispatcher(d.Webhooks, cfg.Webhooks.PollInterval).Run,
		},
	}
}

// HTTPAddr is the address the HTTP server is listening on, or "" before Start.
func (s *Server) HTTPAddr() string {
	if s.httpListener == nil {
		return ""
	}
	return s.httpListener.Addr().String()
}

// GRPCAddr is the address the gRPC server is listening on, or "" before Start.
func (s *Server) GRPCAddr() string {
	return s.grpc.Addr()
}

// Start binds both listeners in the foreground, so a port conflict is returned to
// the caller, then serves and starts the background loops.
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.http.Addr, err)
	}
	if err := s.grpc.Listen(); err != nil {
		_ = lis.Close()
		return err
	}
	s.httpListener = lis

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	for _, loop := range s.loops {
		s.wg.Go(func() { loop(ctx) })
	}
	go func() {
		if err := s.grpc.Serve(); err != nil {
			slog.Error("gRPC server stopped", "err", err)
		}
	}()
	go func() {
		slog.Info("HTTP server listening", "addr", s.HTTPAddr())
		if err := s.http.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server stopped", "err", err)
		}
	}()
	return nil
}

// Shutdown stops the loops and both servers, and waits for the loops to return.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("shutting down")
	s.cancel()
	s.grpc.Stop(ctx)
	err := s.http.Shutdown(ctx)
	s.wg.Wait()
	return err
}
