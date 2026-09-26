package grpc

import (
	"context"
	"errors"
	"log/slog"
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

type JobResultCallback func(workerID string, result *pb.JobResult)

type WorkerStatusCallback func(conn *WorkerConnection, st *pb.WorkerStatus)

// WorkerConnection is the bidirectional stream of one connected worker.
type WorkerConnection struct {
	WorkerID    string
	Queues      []string
	Concurrency int32
	ActiveJobs  int32
	Labels      map[string]string // loaded at registration, updated by UpdateWorkerLabels

	stream   pb.WorkerService_ConnectServer
	pingCh   chan pingRequest
	jobCh    chan *pb.JobAssignment
	drainCh  chan *pb.DrainSignal
	logReqCh chan *pb.JobLogRequest

	logMu      sync.Mutex
	logWaiters map[string]chan jobLogResult

	mu       sync.Mutex
	draining bool

	closeOnce sync.Once
	closed    chan struct{}
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
		closed:      make(chan struct{}),
	}
}

func (wc *WorkerConnection) setCapacity(queues []string, concurrency int32) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.Queues = queues
	if concurrency > 0 {
		wc.Concurrency = concurrency
	}
}

// Close ends Run, and with it the worker's stream.
func (wc *WorkerConnection) Close() {
	wc.closeOnce.Do(func() { close(wc.closed) })
}

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

// AssignJob reserves capacity and queues the assignment for the Run loop.
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

// Shutdown tells a deleted worker to finish its in-flight jobs, report drained, and stop.
func (wc *WorkerConnection) Shutdown() {
	wc.markDraining()
	select {
	case wc.drainCh <- &pb.DrainSignal{Shutdown: true}:
	default:
	}
}

func (wc *WorkerConnection) markDraining() {
	wc.mu.Lock()
	wc.draining = true
	wc.mu.Unlock()
}

func (wc *WorkerConnection) IsAvailable() bool {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	return !wc.draining && wc.ActiveJobs < wc.Concurrency
}

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

// Run pumps both directions of the stream until it closes.
func (wc *WorkerConnection) Run(onJobResult JobResultCallback, onStatus WorkerStatusCallback) {
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

		case <-wc.closed:
			wc.failAllLogWaiters(ErrNoConnection)
			return

		case err := <-recvErrCh:
			if pendingPing != nil {
				pendingPing.resultCh <- pingResult{Err: err}
			}
			wc.failAllLogWaiters(err)
			slog.Warn("worker stream recv", "worker_id", wc.WorkerID, "err", err)
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
			if st := msg.GetStatus(); st != nil {
				// A draining agent refuses new work; stop offering it any.
				if st.GetDraining() || st.GetDrained() {
					wc.markDraining()
				}
				if onStatus != nil {
					onStatus(wc, st)
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
				slog.Warn("send job to worker", "worker_id", wc.WorkerID, "err", err)
				return
			}

		case sig := <-wc.drainCh:
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
				slog.Warn("send job log request to worker", "worker_id", wc.WorkerID, "err", err)
				return
			}
		}
	}
}

type ConnectionManager struct {
	mu    sync.RWMutex
	conns map[string]*WorkerConnection
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		conns: make(map[string]*WorkerConnection),
	}
}

func (cm *ConnectionManager) Register(workerID string, stream pb.WorkerService_ConnectServer) *WorkerConnection {
	wc := newWorkerConnection(workerID, stream)
	cm.mu.Lock()
	cm.conns[workerID] = wc
	cm.mu.Unlock()
	return wc
}

// Unregister drops conn unless a newer stream for the same worker has replaced it.
func (cm *ConnectionManager) Unregister(conn *WorkerConnection) {
	cm.mu.Lock()
	if cm.conns[conn.WorkerID] == conn {
		delete(cm.conns, conn.WorkerID)
	}
	cm.mu.Unlock()
}

func (cm *ConnectionManager) Get(workerID string) *WorkerConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.conns[workerID]
}

func (cm *ConnectionManager) ConnectedWorkerIDs() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	ids := make([]string, 0, len(cm.conns))
	for id := range cm.conns {
		ids = append(ids, id)
	}
	return ids
}

// FindAvailableWorker returns a worker on queueName with free capacity whose labels
// include every selector pair; an empty selector matches any worker.
func (cm *ConnectionManager) FindAvailableWorker(queueName string, selector map[string]string) *WorkerConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var best *WorkerConnection
	var bestLoad int32
	for _, wc := range cm.conns {
		wc.mu.Lock()
		draining := wc.draining
		active := wc.ActiveJobs
		limit := wc.Concurrency
		var queueMatch bool
		for _, q := range wc.Queues {
			if q == queueName {
				queueMatch = true
				break
			}
		}
		labelMatch := workerMatchesSelector(wc.Labels, selector)
		wc.mu.Unlock()

		if draining || !queueMatch || !labelMatch || active >= limit {
			continue
		}
		if best == nil || active < bestLoad {
			best = wc
			bestLoad = active
		}
	}
	return best
}

func workerMatchesSelector(workerLabels, selector map[string]string) bool {
	for k, v := range selector {
		if workerLabels[k] != v {
			return false
		}
	}
	return true
}

func (cm *ConnectionManager) UpdateWorkerLabels(workerID string, labels map[string]string) {
	cm.mu.RLock()
	wc := cm.conns[workerID]
	cm.mu.RUnlock()
	if wc == nil {
		return
	}
	wc.mu.Lock()
	wc.Labels = labels
	wc.mu.Unlock()
}

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

type WorkerInfo struct {
	WorkerID    string   `json:"worker_id"`
	Queues      []string `json:"queues"`
	Concurrency int32    `json:"concurrency"`
	ActiveJobs  int32    `json:"active_jobs"`
	Draining    bool     `json:"draining"`
}
