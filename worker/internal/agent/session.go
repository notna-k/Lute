package agent

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	pb "github.com/lute/proto"

	"github.com/lute/worker/internal/joblog"
	"github.com/lute/worker/internal/metrics"
)

// session serves one open stream to the core; jobs belong to the agent and outlive it.
type session struct {
	agent  *Agent
	stream pb.WorkerService_ConnectClient
	sendMu sync.Mutex // job goroutines and the receive loop share the stream
}

func (s *session) send(msg *pb.WorkerMessage) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return s.stream.Send(msg)
}

func (s *session) closeSend() error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	return s.stream.CloseSend()
}

func (s *session) serve() error {
	a := s.agent
	for {
		msg, err := s.stream.Recv()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}
		if msg.GetHeartbeatPing() != nil {
			if err := s.send(a.pong()); err != nil {
				return fmt.Errorf("send pong: %w", err)
			}
		}
		if req := msg.GetJobLogRequest(); req != nil {
			go s.answerLogRequest(req)
		}
		if as := msg.GetAssign(); as != nil {
			a.accept(as)
		}
		if msg.GetDrain() != nil {
			a.deleteRequested()
		}
	}
}

func (a *Agent) pong() *pb.WorkerMessage {
	values := make(map[string]*pb.MetricValue)
	for k, v := range metrics.Collect() {
		values[k] = &pb.MetricValue{Kind: &pb.MetricValue_F{F: v}}
	}
	state := "running"
	a.mu.Lock()
	if a.draining {
		state = "draining"
	}
	a.mu.Unlock()
	return &pb.WorkerMessage{Payload: &pb.WorkerMessage_HeartbeatPong{HeartbeatPong: &pb.HeartbeatPong{
		Status:    state,
		Metrics:   values,
		Timestamp: time.Now().Unix(),
	}}}
}

func (s *session) answerLogRequest(req *pb.JobLogRequest) {
	resp := s.agent.cfg.Jobs.ReadLog(req)
	if err := s.send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_JobLogResponse{JobLogResponse: resp}}); err != nil {
		slog.Error("Failed to send job log response", "request_id", req.RequestId, "err", err)
	}
}

// accept runs an assigned job in the background, or refuses it while draining. The check
// and the WaitGroup Add share the lock, so a drain never misses a job it let in.
func (a *Agent) accept(as *pb.JobAssignment) {
	a.mu.Lock()
	if a.draining {
		a.mu.Unlock()
		a.sendResult(&pb.JobResult{JobId: as.JobId, Error: "worker is draining"})
		return
	}
	a.jobs.Add(1)
	a.mu.Unlock()

	go func() {
		defer a.jobs.Done()
		start := time.Now()
		err := a.cfg.Jobs.Execute(a.jobCtx, as)
		result := &pb.JobResult{
			JobId:     as.JobId,
			Success:   err == nil,
			ElapsedMs: time.Since(start).Milliseconds(),
			LogFile:   joblog.FileName(as.JobId),
		}
		if err != nil {
			result.Error = err.Error()
			slog.Warn("Job failed", "job_id", as.JobId, "err", err)
		} else {
			slog.Info("Job completed", "job_id", as.JobId, "elapsed_ms", result.ElapsedMs)
		}
		a.sendResult(result)
	}()
}

// sendResult reports on whichever stream is open; with none, core's lease reaper settles the job.
func (a *Agent) sendResult(r *pb.JobResult) {
	s := a.session()
	if s == nil {
		slog.Error("No connection to report a job result on", "job_id", r.JobId)
		return
	}
	if err := s.send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_Result{Result: r}}); err != nil {
		slog.Error("Failed to send job result", "job_id", r.JobId, "err", err)
	}
}
