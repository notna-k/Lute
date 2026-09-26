//go:build e2e

package harness

import (
	"context"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/lute/proto"
)

// Register calls the Register RPC as an agent's first start does.
func (s *Stack) Register(token, name string) (*pb.RegisterResponse, error) {
	conn, err := grpc.NewClient(s.GRPCAddr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return pb.NewWorkerServiceClient(conn).Register(ctx, &pb.RegisterRequest{
		Token:    token,
		Name:     name,
		Version:  "e2e",
		Protocol: pb.Protocol,
		Queues:   []string{"default"},
	})
}

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

// DialRawWorker opens a stream with workerID and secret as its credential; it sends nothing.
func (s *Stack) DialRawWorker(workerID, secret string) (*RawWorker, error) {
	s.t.Helper()

	conn, err := grpc.NewClient(s.GRPCAddr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+workerID+"."+secret)
	stream, err := pb.NewWorkerServiceClient(conn).Connect(ctx)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	w := &RawWorker{t: s.t, WorkerID: workerID, conn: conn, stream: stream}
	go w.receive()
	s.t.Cleanup(w.Close)
	return w, nil
}

// Register sends the registration every stream must open with.
func (w *RawWorker) Register(concurrency int32, queues ...string) error {
	return w.send(&pb.WorkerMessage{
		Payload: &pb.WorkerMessage_Register{
			Register: &pb.WorkerRegistration{Queues: queues, Concurrency: concurrency, Version: "e2e", Protocol: pb.Protocol},
		},
	})
}

// RegisterRunning opens the stream as an agent that reconnected while jobIDs kept running.
func (w *RawWorker) RegisterRunning(concurrency int32, queue string, jobIDs ...string) error {
	return w.send(&pb.WorkerMessage{
		Payload: &pb.WorkerMessage_Register{
			Register: &pb.WorkerRegistration{Queues: []string{queue}, Concurrency: concurrency, Version: "e2e", Protocol: pb.Protocol, RunningJobs: jobIDs},
		},
	})
}

// RegisterWithProtocol opens the stream claiming an arbitrary protocol version.
func (w *RawWorker) RegisterWithProtocol(protocol int32) error {
	return w.send(&pb.WorkerMessage{
		Payload: &pb.WorkerMessage_Register{
			Register: &pb.WorkerRegistration{Queues: []string{"default"}, Concurrency: 1, Version: "0.0.1", Protocol: protocol},
		},
	})
}

func (w *RawWorker) ReportResult(jobID string, success bool, errMsg string, elapsedMs int64) error {
	return w.send(&pb.WorkerMessage{
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

func (w *RawWorker) ReportStatus(st *pb.WorkerStatus) error {
	return w.send(&pb.WorkerMessage{Payload: &pb.WorkerMessage_Status{Status: st}})
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
