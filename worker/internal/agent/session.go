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
	a.running[as.JobId] = struct{}{}
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
		a.finishJob(result)
	}()
}

// finishJob queues a job's result and delivers it on the open stream, or on the next one.
func (a *Agent) finishJob(r *pb.JobResult) {
	a.mu.Lock()
	delete(a.running, r.JobId)
	a.pending = append(a.pending, r)
	a.mu.Unlock()
	a.flushPending()
}

// flushPending sends queued results on the current stream. What fails to send goes back
// to the queue; if a new stream took over meanwhile, it may have flushed the queue before
// the failure was queued again, so the loop tries once more on that one.
func (a *Agent) flushPending() {
	for {
		a.mu.Lock()
		s, batch := a.sess, a.pending
		if s == nil || len(batch) == 0 {
			a.mu.Unlock()
			return
		}
		a.pending = nil
		a.mu.Unlock()

		for i, r := range batch {
			if err := s.send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_Result{Result: r}}); err != nil {
				slog.Warn("Failed to send job result; retrying on the next connection", "job_id", r.JobId, "err", err)
				a.mu.Lock()
				a.pending = append(a.pending, batch[i:]...)
				retry := a.sess != s
				a.mu.Unlock()
				if !retry {
					return
				}
				break
			}
		}
		// Loop: results may have been queued while this batch was sent.
	}
}

// sendResult reports a refusal; it is not kept, since core requeues on its own.
func (a *Agent) sendResult(r *pb.JobResult) {
	s := a.session()
	if s == nil {
		return
	}
	if err := s.send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_Result{Result: r}}); err != nil {
		slog.Error("Failed to send job result", "job_id", r.JobId, "err", err)
	}
}
