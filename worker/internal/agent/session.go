package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	pb "github.com/lute/proto"

	"github.com/lute/worker/internal/joblog"
	"github.com/lute/worker/internal/metrics"
)

// session serves one open stream to the core.
type session struct {
	cfg    Config
	stream pb.WorkerService_ConnectClient

	sendMu sync.Mutex // job goroutines and the receive loop share the stream
	jobs   sync.WaitGroup

	draining     bool
	shuttingDown bool
}

func (s *session) send(msg *pb.WorkerMessage) error {
	msg.WorkerId = s.cfg.WorkerID
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return s.stream.Send(msg)
}

func (s *session) serve(ctx context.Context) error {
	for {
		msg, err := s.stream.Recv()
		if err != nil {
			if s.shuttingDown {
				s.jobs.Wait()
				return errShutdown
			}
			return fmt.Errorf("recv: %w", err)
		}

		if msg.GetHeartbeatPing() != nil {
			if err := s.send(pong()); err != nil {
				return fmt.Errorf("send pong: %w", err)
			}
		}
		if req := msg.GetJobLogRequest(); req != nil {
			go s.answerLogRequest(req)
		}
		if a := msg.GetAssign(); a != nil {
			s.accept(ctx, a)
		}
		if d := msg.GetDrain(); d != nil {
			s.drain(d)
		}
	}
}

func pong() *pb.WorkerMessage {
	values := make(map[string]*pb.MetricValue)
	for k, v := range metrics.Collect() {
		values[k] = &pb.MetricValue{Kind: &pb.MetricValue_F{F: v}}
	}
	return &pb.WorkerMessage{Payload: &pb.WorkerMessage_HeartbeatPong{HeartbeatPong: &pb.HeartbeatPong{
		Status:    "running",
		Metrics:   values,
		Timestamp: time.Now().Unix(),
	}}}
}

func (s *session) answerLogRequest(req *pb.JobLogRequest) {
	resp := readJobLog(s.cfg.JobLogsDir, req)
	if err := s.send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_JobLogResponse{JobLogResponse: resp}}); err != nil {
		slog.Error("Failed to send job log response", "request_id", req.RequestId, "err", err)
	}
}

// accept runs an assigned job in the background, or refuses it while draining.
func (s *session) accept(ctx context.Context, a *pb.JobAssignment) {
	if s.draining {
		_ = s.sendResult(&pb.JobResult{JobId: a.JobId, Error: "worker is draining"})
		return
	}

	s.jobs.Go(func() {
		start := time.Now()
		err := execute(ctx, s.cfg.JobLogsDir, a)
		result := &pb.JobResult{
			JobId:     a.JobId,
			Success:   err == nil,
			ElapsedMs: time.Since(start).Milliseconds(),
		}
		if s.cfg.JobLogsDir != "" {
			result.LogFile = joblog.FileName(a.JobId)
		}
		if err != nil {
			result.Error = err.Error()
			slog.Warn("Job failed", "job_id", a.JobId, "err", err)
		} else {
			slog.Info("Job completed", "job_id", a.JobId, "elapsed_ms", result.ElapsedMs)
		}
		if err := s.sendResult(result); err != nil {
			slog.Error("Failed to send job result", "job_id", a.JobId, "err", err)
		}
	})
}

func (s *session) sendResult(r *pb.JobResult) error {
	return s.send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_Result{Result: r}})
}

// drain stops taking jobs; with shutdown set, it also closes the stream once in-flight jobs finish.
func (s *session) drain(d *pb.DrainSignal) {
	s.draining = true
	if !d.GetShutdown() {
		slog.Info("Drain signal received, finishing in-flight jobs")
		return
	}
	if s.shuttingDown {
		return
	}
	s.shuttingDown = true
	slog.Info("Shutdown signal received, finishing in-flight jobs then exiting")
	go func() {
		s.jobs.Wait()
		_ = s.stream.CloseSend()
	}()
}
