//go:build e2e

package harness

import (
	"context"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/lute/proto"
)

// RawWorker speaks the protocol directly, for what a correct agent never does: results
// for builds it was not given or that core gave up on, or a second stream for one worker.
// Anything a well-behaved worker does belongs in a real Agent.
type RawWorker struct {
	t        *testing.T
	WorkerID string

	conn   *grpc.ClientConn
	stream pb.WorkerService_ConnectClient

	sendMu sync.Mutex

	mu          sync.Mutex
	assignments []*pb.JobAssignment
	logRequests []*pb.JobLogRequest
	pings       int
	drains      []*pb.DrainSignal
	recvErr     error
}

// DialRawWorker sends the identifying first message but does not Register.
func (s *Stack) DialRawWorker(workerID string) (*RawWorker, error) {
	s.t.Helper()

	conn, err := grpc.NewClient(s.GRPCAddr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	stream, err := pb.NewWorkerServiceClient(conn).Connect(context.Background())
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	w := &RawWorker{t: s.t, WorkerID: workerID, conn: conn, stream: stream}
	if err := stream.Send(&pb.WorkerMessage{WorkerId: workerID}); err != nil {
		w.Close()
		return nil, err
	}
	go w.receive()
	s.t.Cleanup(w.Close)
	return w, nil
}

func (w *RawWorker) Register(concurrency int32, queues ...string) error {
	return w.send(&pb.WorkerMessage{
		WorkerId: w.WorkerID,
		Payload: &pb.WorkerMessage_Register{
			Register: &pb.WorkerRegistration{Queues: queues, Concurrency: concurrency},
		},
	})
}

func (w *RawWorker) ReportResult(jobID string, success bool, errMsg string, elapsedMs int64) error {
	return w.send(&pb.WorkerMessage{
		WorkerId: w.WorkerID,
		Payload: &pb.WorkerMessage_Result{
			Result: &pb.JobResult{
				JobId:     jobID,
				Success:   success,
				Error:     errMsg,
				ElapsedMs: elapsedMs,
			},
		},
	})
}

func (w *RawWorker) Close() {
	if w.conn != nil {
		_ = w.conn.Close()
		w.conn = nil
	}
}

func (w *RawWorker) Assignments() []*pb.JobAssignment {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]*pb.JobAssignment(nil), w.assignments...)
}

func (w *RawWorker) WaitForAssignment(timeout time.Duration) *pb.JobAssignment {
	w.t.Helper()
	return Eventually(w.t, timeout, "an assignment for worker "+w.WorkerID,
		func() (*pb.JobAssignment, bool) {
			got := w.Assignments()
			if len(got) == 0 {
				return nil, false
			}
			return got[len(got)-1], true
		})
}

func (w *RawWorker) Drains() []*pb.DrainSignal {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]*pb.DrainSignal(nil), w.drains...)
}

func (w *RawWorker) RecvError() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.recvErr
}

func (w *RawWorker) WaitForStreamEnd(timeout time.Duration) error {
	w.t.Helper()
	return Eventually(w.t, timeout, "core to end the stream for "+w.WorkerID,
		func() (error, bool) {
			err := w.RecvError()
			return err, err != nil
		})
}

func (w *RawWorker) send(msg *pb.WorkerMessage) error {
	w.sendMu.Lock()
	defer w.sendMu.Unlock()
	return w.stream.Send(msg)
}

func (w *RawWorker) receive() {
	for {
		msg, err := w.stream.Recv()
		if err != nil {
			w.mu.Lock()
			w.recvErr = err
			w.mu.Unlock()
			return
		}
		w.mu.Lock()
		if a := msg.GetAssign(); a != nil {
			w.assignments = append(w.assignments, a)
		}
		if msg.GetHeartbeatPing() != nil {
			w.pings++
		}
		if d := msg.GetDrain(); d != nil {
			w.drains = append(w.drains, d)
		}
		if lr := msg.GetJobLogRequest(); lr != nil {
			w.logRequests = append(w.logRequests, lr)
		}
		w.mu.Unlock()
	}
}
