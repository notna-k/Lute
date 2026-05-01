package grpc

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	pb "github.com/lute/proto"
)

var (
	ErrNoConnection = errors.New("no active connection for worker")
	ErrPingTimeout  = errors.New("heartbeat ping timed out")
)

type jobLogResult struct {
	Resp *pb.JobLogResponse
	Err  error
}

type pingRequest struct {
	resultCh chan<- pingResult
}

type pingResult struct {
	Pong *pb.HeartbeatPong
	Err  error
}

// JobResultCallback is called when a worker reports a job result.
type JobResultCallback func(workerID string, result *pb.JobResult)

// WorkerRegistrationCallback is called when a worker sends registration info.
type WorkerRegistrationCallback func(workerID string, reg *pb.WorkerRegistration)

// WorkerConnection wraps a single bidirectional stream for one connected worker.
type WorkerConnection struct {
	WorkerID    string
	Queues      []string
	Concurrency int32
	ActiveJobs  int32

	stream   pb.WorkerService_ConnectServer
	pingCh   chan pingRequest
	jobCh    chan *pb.JobAssignment
	drainCh  chan *pb.DrainSignal
	logReqCh chan *pb.JobLogRequest

	logMu      sync.Mutex
	logWaiters map[string]chan jobLogResult

	mu       sync.Mutex
	draining bool
}

func newWorkerConnection(workerID string, stream pb.WorkerService_ConnectServer) *WorkerConnection {
	return &WorkerConnection{
		WorkerID:    workerID,
		Concurrency: 1,
		stream:      stream,
		pingCh:      make(chan pingRequest, 1),
		jobCh:       make(chan *pb.JobAssignment, 1024),
		drainCh:     make(chan *pb.DrainSignal, 1),
		logReqCh:    make(chan *pb.JobLogRequest, 32),
		logWaiters:  make(map[string]chan jobLogResult),
	}
}

// Ping sends a HeartbeatPing over the stream and waits for the pong.
func (wc *WorkerConnection) Ping(timeout time.Duration) (*pb.HeartbeatPong, error) {
	resultCh := make(chan pingResult, 1)
	select {
	case wc.pingCh <- pingRequest{resultCh: resultCh}:
	case <-time.After(timeout):
		return nil, ErrPingTimeout
	}
	select {
	case res := <-resultCh:
		return res.Pong, res.Err
	case <-time.After(timeout):
		return nil, ErrPingTimeout
	}
}

// AssignJob reserves worker capacity and queues the assignment for the Run loop.
func (wc *WorkerConnection) AssignJob(assignment *pb.JobAssignment) bool {
	wc.mu.Lock()
	if wc.draining || wc.ActiveJobs >= wc.Concurrency {
		wc.mu.Unlock()
		return false
	}
	wc.ActiveJobs++
	wc.mu.Unlock()

	select {
	case wc.jobCh <- assignment:
		return true
	default:
		wc.mu.Lock()
		wc.ActiveJobs--
		wc.mu.Unlock()
		return false
	}
}

// Drain signals the worker to stop accepting new jobs.
func (wc *WorkerConnection) Drain() {
	wc.sendDrain(&pb.DrainSignal{})
}

// Shutdown signals the worker to stop accepting new jobs and exit its process
// once in-flight jobs complete.
func (wc *WorkerConnection) Shutdown() {
	wc.sendDrain(&pb.DrainSignal{Shutdown: true})
}

func (wc *WorkerConnection) sendDrain(sig *pb.DrainSignal) {
	wc.mu.Lock()
	wc.draining = true
	wc.mu.Unlock()
	select {
	case wc.drainCh <- sig:
	default:
	}
}

// IsAvailable returns true if the worker can accept more jobs.
func (wc *WorkerConnection) IsAvailable() bool {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	return !wc.draining && wc.ActiveJobs < wc.Concurrency
}

// RequestJobLog sends a JobLogRequest on the stream and waits for JobLogResponse.
func (wc *WorkerConnection) RequestJobLog(ctx context.Context, req *pb.JobLogRequest) (*pb.JobLogResponse, error) {
	if req.RequestId == "" {
		req.RequestId = uuid.New().String()
	}
	resultCh := make(chan jobLogResult, 1)

	wc.logMu.Lock()
	wc.logWaiters[req.RequestId] = resultCh
	wc.logMu.Unlock()

	defer func() {
		wc.logMu.Lock()
		if wc.logWaiters[req.RequestId] == resultCh {
			delete(wc.logWaiters, req.RequestId)
		}
		wc.logMu.Unlock()
	}()

	select {
	case wc.logReqCh <- req:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case res := <-resultCh:
		return res.Resp, res.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (wc *WorkerConnection) finishLogWaiter(requestID string, res jobLogResult) {
	wc.logMu.Lock()
	ch, ok := wc.logWaiters[requestID]
	if ok {
		delete(wc.logWaiters, requestID)
	}
	wc.logMu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- res:
	default:
	}
}

func (wc *WorkerConnection) failAllLogWaiters(err error) {
	wc.logMu.Lock()
	waiters := wc.logWaiters
	wc.logWaiters = make(map[string]chan jobLogResult)
	wc.logMu.Unlock()
	for _, ch := range waiters {
		select {
		case ch <- jobLogResult{Err: err}:
		default:
		}
	}
}

// Run processes outgoing and incoming messages on the bidirectional stream.
func (wc *WorkerConnection) Run(onJobResult JobResultCallback, onRegistration WorkerRegistrationCallback) {
	recvCh := make(chan *pb.WorkerMessage, 1)
	recvErrCh := make(chan error, 1)

	go func() {
		for {
			msg, err := wc.stream.Recv()
			if err != nil {
				recvErrCh <- err
				return
			}
			recvCh <- msg
		}
	}()

	var pendingPing *pingRequest

	for {
		select {
		case <-wc.stream.Context().Done():
			wc.failAllLogWaiters(wc.stream.Context().Err())
			return

		case err := <-recvErrCh:
			if pendingPing != nil {
				pendingPing.resultCh <- pingResult{Err: err}
			}
			wc.failAllLogWaiters(err)
			log.Printf("Connection %s recv error: %v", wc.WorkerID, err)
			return

		case msg := <-recvCh:
			if pong := msg.GetHeartbeatPong(); pong != nil && pendingPing != nil {
				pendingPing.resultCh <- pingResult{Pong: pong}
				pendingPing = nil
			}
			if lr := msg.GetJobLogResponse(); lr != nil {
				wc.finishLogWaiter(lr.RequestId, jobLogResult{Resp: lr})
			}
			if result := msg.GetResult(); result != nil {
				wc.mu.Lock()
				wc.ActiveJobs--
				wc.mu.Unlock()
				if onJobResult != nil {
					onJobResult(wc.WorkerID, result)
				}
			}
			if reg := msg.GetRegister(); reg != nil {
				wc.mu.Lock()
				wc.Queues = reg.Queues
				if reg.Concurrency > 0 {
					wc.Concurrency = reg.Concurrency
				}
				wc.mu.Unlock()
				if onRegistration != nil {
					onRegistration(wc.WorkerID, reg)
				}
			}

		case req := <-wc.pingCh:
			err := wc.stream.Send(&pb.ServerMessage{
				Payload: &pb.ServerMessage_HeartbeatPing{
					HeartbeatPing: &pb.HeartbeatPing{
						Timestamp: time.Now().Unix(),
					},
				},
			})
			if err != nil {
				req.resultCh <- pingResult{Err: err}
				return
			}
			pendingPing = &req

		case assignment := <-wc.jobCh:
			err := wc.stream.Send(&pb.ServerMessage{
				Payload: &pb.ServerMessage_Assign{
					Assign: assignment,
				},
			})
			if err != nil {
				wc.mu.Lock()
				wc.ActiveJobs--
				wc.mu.Unlock()
				log.Printf("Connection %s send job error: %v", wc.WorkerID, err)
				return
			}

		case sig := <-wc.drainCh:
			if sig == nil {
				sig = &pb.DrainSignal{}
			}
			_ = wc.stream.Send(&pb.ServerMessage{
				Payload: &pb.ServerMessage_Drain{
					Drain: sig,
				},
			})

		case logReq := <-wc.logReqCh:
			err := wc.stream.Send(&pb.ServerMessage{
				Payload: &pb.ServerMessage_JobLogRequest{
					JobLogRequest: logReq,
				},
			})
			if err != nil {
				wc.finishLogWaiter(logReq.RequestId, jobLogResult{Err: err})
				log.Printf("Connection %s send job log request error: %v", wc.WorkerID, err)
				return
			}
		}
	}
}

// ConnectionManager tracks active bidirectional streams keyed by worker ID.
type ConnectionManager struct {
	mu    sync.RWMutex
	conns map[string]*WorkerConnection
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		conns: make(map[string]*WorkerConnection),
	}
}

// Register adds (or replaces) a connection for the given worker.
func (cm *ConnectionManager) Register(workerID string, stream pb.WorkerService_ConnectServer) *WorkerConnection {
	wc := newWorkerConnection(workerID, stream)
	cm.mu.Lock()
	cm.conns[workerID] = wc
	cm.mu.Unlock()
	return wc
}

// Unregister removes the connection for a worker.
func (cm *ConnectionManager) Unregister(workerID string) {
	cm.mu.Lock()
	delete(cm.conns, workerID)
	cm.mu.Unlock()
}

// Get returns the active connection for a worker, or nil.
func (cm *ConnectionManager) Get(workerID string) *WorkerConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.conns[workerID]
}

// ConnectedWorkerIDs returns a snapshot of all connected worker IDs.
func (cm *ConnectionManager) ConnectedWorkerIDs() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	ids := make([]string, 0, len(cm.conns))
	for id := range cm.conns {
		ids = append(ids, id)
	}
	return ids
}

// FindAvailableWorker returns a connected worker that handles the given queue
// and has capacity for another job.
func (cm *ConnectionManager) FindAvailableWorker(queueName string) *WorkerConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var best *WorkerConnection
	var bestLoad int32
	for _, wc := range cm.conns {
		wc.mu.Lock()
		draining := wc.draining
		active := wc.ActiveJobs
		limit := wc.Concurrency
		var match bool
		for _, q := range wc.Queues {
			if q == queueName {
				match = true
				break
			}
		}
		wc.mu.Unlock()

		if draining || !match || active >= limit {
			continue
		}
		if best == nil || active < bestLoad {
			best = wc
			bestLoad = active
		}
	}
	return best
}

// ActiveWorkers returns info about all connected workers.
func (cm *ConnectionManager) ActiveWorkers() []WorkerInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	out := make([]WorkerInfo, 0, len(cm.conns))
	for _, wc := range cm.conns {
		wc.mu.Lock()
		out = append(out, WorkerInfo{
			WorkerID:    wc.WorkerID,
			Queues:      wc.Queues,
			Concurrency: wc.Concurrency,
			ActiveJobs:  wc.ActiveJobs,
			Draining:    wc.draining,
		})
		wc.mu.Unlock()
	}
	return out
}

// WorkerInfo holds summary data about a connected worker.
type WorkerInfo struct {
	WorkerID    string   `json:"worker_id"`
	Queues      []string `json:"queues"`
	Concurrency int32    `json:"concurrency"`
	ActiveJobs  int32    `json:"active_jobs"`
	Draining    bool     `json:"draining"`
}
