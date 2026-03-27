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
	ErrNoConnection = errors.New("no active connection for machine")
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
type JobResultCallback func(machineID string, result *pb.JobResult)

// WorkerRegistrationCallback is called when a worker sends registration info.
type WorkerRegistrationCallback func(machineID string, reg *pb.WorkerRegistration)

// MachineConnection wraps a single bidirectional stream for one machine/worker.
// The Run loop handles multiplexed messages: heartbeat pings, job assignments,
// and incoming results/pongs.
type MachineConnection struct {
	MachineID   string
	Queues      []string
	Concurrency int32
	ActiveJobs  int32

	stream  pb.WorkerService_ConnectServer
	pingCh  chan pingRequest
	jobCh   chan *pb.JobAssignment
	drainCh chan struct{}
	logReqCh chan *pb.JobLogRequest

	logMu      sync.Mutex
	logWaiters map[string]chan jobLogResult

	mu       sync.Mutex
	draining bool
}

func newMachineConnection(machineID string, stream pb.WorkerService_ConnectServer) *MachineConnection {
	return &MachineConnection{
		MachineID:   machineID,
		Concurrency: 1,
		stream:      stream,
		pingCh:      make(chan pingRequest, 1),
		// Large enough that a reserved slot (see AssignJob) can always be queued before Run sends it.
		jobCh:    make(chan *pb.JobAssignment, 1024),
		drainCh:  make(chan struct{}, 1),
		logReqCh: make(chan *pb.JobLogRequest, 32),
		logWaiters: make(map[string]chan jobLogResult),
	}
}

// Ping sends a HeartbeatPing over the stream and waits for the pong.
func (mc *MachineConnection) Ping(timeout time.Duration) (*pb.HeartbeatPong, error) {
	resultCh := make(chan pingResult, 1)
	select {
	case mc.pingCh <- pingRequest{resultCh: resultCh}:
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

// AssignJob reserves worker capacity and queues the assignment for the Run loop
// to send on the stream. Non-blocking on jobCh; returns false if draining, at
// capacity, or the outbound queue is full.
func (mc *MachineConnection) AssignJob(assignment *pb.JobAssignment) bool {
	mc.mu.Lock()
	if mc.draining || mc.ActiveJobs >= mc.Concurrency {
		mc.mu.Unlock()
		return false
	}
	mc.ActiveJobs++
	mc.mu.Unlock()

	select {
	case mc.jobCh <- assignment:
		return true
	default:
		mc.mu.Lock()
		mc.ActiveJobs--
		mc.mu.Unlock()
		return false
	}
}

// Drain signals the worker to stop accepting new jobs.
func (mc *MachineConnection) Drain() {
	mc.mu.Lock()
	mc.draining = true
	mc.mu.Unlock()
	select {
	case mc.drainCh <- struct{}{}:
	default:
	}
}

// IsAvailable returns true if the worker can accept more jobs.
func (mc *MachineConnection) IsAvailable() bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return !mc.draining && mc.ActiveJobs < mc.Concurrency
}

// RequestJobLog sends a JobLogRequest on the stream and waits for JobLogResponse.
// If req.RequestId is empty, a UUID is assigned. ctx controls total wait time.
func (mc *MachineConnection) RequestJobLog(ctx context.Context, req *pb.JobLogRequest) (*pb.JobLogResponse, error) {
	if req.RequestId == "" {
		req.RequestId = uuid.New().String()
	}
	resultCh := make(chan jobLogResult, 1)

	mc.logMu.Lock()
	mc.logWaiters[req.RequestId] = resultCh
	mc.logMu.Unlock()

	defer func() {
		mc.logMu.Lock()
		if mc.logWaiters[req.RequestId] == resultCh {
			delete(mc.logWaiters, req.RequestId)
		}
		mc.logMu.Unlock()
	}()

	select {
	case mc.logReqCh <- req:
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

func (mc *MachineConnection) finishLogWaiter(requestID string, res jobLogResult) {
	mc.logMu.Lock()
	ch, ok := mc.logWaiters[requestID]
	if ok {
		delete(mc.logWaiters, requestID)
	}
	mc.logMu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- res:
	default:
	}
}

func (mc *MachineConnection) failAllLogWaiters(err error) {
	mc.logMu.Lock()
	waiters := mc.logWaiters
	mc.logWaiters = make(map[string]chan jobLogResult)
	mc.logMu.Unlock()
	for _, ch := range waiters {
		select {
		case ch <- jobLogResult{Err: err}:
		default:
		}
	}
}

// Run processes outgoing (pings, job assignments, drain signals) and incoming
// messages (pongs, job results, registration) on the bidirectional stream.
// It blocks until the stream closes.
func (mc *MachineConnection) Run(onJobResult JobResultCallback, onRegistration WorkerRegistrationCallback) {
	recvCh := make(chan *pb.WorkerMessage, 1)
	recvErrCh := make(chan error, 1)

	go func() {
		for {
			msg, err := mc.stream.Recv()
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
		case <-mc.stream.Context().Done():
			mc.failAllLogWaiters(mc.stream.Context().Err())
			return

		case err := <-recvErrCh:
			if pendingPing != nil {
				pendingPing.resultCh <- pingResult{Err: err}
			}
			mc.failAllLogWaiters(err)
			log.Printf("Connection %s recv error: %v", mc.MachineID, err)
			return

		case msg := <-recvCh:
			if pong := msg.GetHeartbeatPong(); pong != nil && pendingPing != nil {
				pendingPing.resultCh <- pingResult{Pong: pong}
				pendingPing = nil
			}
			if lr := msg.GetJobLogResponse(); lr != nil {
				mc.finishLogWaiter(lr.RequestId, jobLogResult{Resp: lr})
			}
			if result := msg.GetResult(); result != nil {
				mc.mu.Lock()
				mc.ActiveJobs--
				mc.mu.Unlock()
				if onJobResult != nil {
					onJobResult(mc.MachineID, result)
				}
			}
			if reg := msg.GetRegister(); reg != nil {
				mc.mu.Lock()
				mc.Queues = reg.Queues
				if reg.Concurrency > 0 {
					mc.Concurrency = reg.Concurrency
				}
				mc.mu.Unlock()
				if onRegistration != nil {
					onRegistration(mc.MachineID, reg)
				}
			}

		case req := <-mc.pingCh:
			err := mc.stream.Send(&pb.ServerMessage{
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

		case assignment := <-mc.jobCh:
			err := mc.stream.Send(&pb.ServerMessage{
				Payload: &pb.ServerMessage_Assign{
					Assign: assignment,
				},
			})
			if err != nil {
				mc.mu.Lock()
				mc.ActiveJobs--
				mc.mu.Unlock()
				log.Printf("Connection %s send job error: %v", mc.MachineID, err)
				return
			}

		case <-mc.drainCh:
			_ = mc.stream.Send(&pb.ServerMessage{
				Payload: &pb.ServerMessage_Drain{
					Drain: &pb.DrainSignal{},
				},
			})

		case logReq := <-mc.logReqCh:
			err := mc.stream.Send(&pb.ServerMessage{
				Payload: &pb.ServerMessage_JobLogRequest{
					JobLogRequest: logReq,
				},
			})
			if err != nil {
				mc.finishLogWaiter(logReq.RequestId, jobLogResult{Err: err})
				log.Printf("Connection %s send job log request error: %v", mc.MachineID, err)
				return
			}
		}
	}
}

// ConnectionManager tracks active bidirectional streams keyed by machine ID.
type ConnectionManager struct {
	mu    sync.RWMutex
	conns map[string]*MachineConnection
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		conns: make(map[string]*MachineConnection),
	}
}

// Register adds (or replaces) a connection for the given machine.
func (cm *ConnectionManager) Register(machineID string, stream pb.WorkerService_ConnectServer) *MachineConnection {
	mc := newMachineConnection(machineID, stream)
	cm.mu.Lock()
	cm.conns[machineID] = mc
	cm.mu.Unlock()
	return mc
}

// Unregister removes the connection for a machine.
func (cm *ConnectionManager) Unregister(machineID string) {
	cm.mu.Lock()
	delete(cm.conns, machineID)
	cm.mu.Unlock()
}

// Get returns the active connection for a machine, or nil.
func (cm *ConnectionManager) Get(machineID string) *MachineConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.conns[machineID]
}

// ConnectedMachineIDs returns a snapshot of all connected machine IDs.
func (cm *ConnectionManager) ConnectedMachineIDs() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	ids := make([]string, 0, len(cm.conns))
	for id := range cm.conns {
		ids = append(ids, id)
	}
	return ids
}

// FindAvailableWorker returns a connected worker that handles the given queue
// and has capacity for another job. When several match, picks the one with the
// fewest active jobs (including assignments not yet sent on the stream).
func (cm *ConnectionManager) FindAvailableWorker(queueName string) *MachineConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var best *MachineConnection
	var bestLoad int32
	for _, mc := range cm.conns {
		mc.mu.Lock()
		draining := mc.draining
		active := mc.ActiveJobs
		limit := mc.Concurrency
		var match bool
		for _, q := range mc.Queues {
			if q == queueName {
				match = true
				break
			}
		}
		mc.mu.Unlock()

		if draining || !match || active >= limit {
			continue
		}
		if best == nil || active < bestLoad {
			best = mc
			bestLoad = active
		}
	}
	return best
}

// ActiveWorkers returns info about all connected workers.
func (cm *ConnectionManager) ActiveWorkers() []WorkerInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	workers := make([]WorkerInfo, 0, len(cm.conns))
	for _, mc := range cm.conns {
		mc.mu.Lock()
		workers = append(workers, WorkerInfo{
			MachineID:   mc.MachineID,
			Queues:      mc.Queues,
			Concurrency: mc.Concurrency,
			ActiveJobs:  mc.ActiveJobs,
			Draining:    mc.draining,
		})
		mc.mu.Unlock()
	}
	return workers
}

// WorkerInfo holds summary data about a connected worker.
type WorkerInfo struct {
	MachineID   string   `json:"machine_id"`
	Queues      []string `json:"queues"`
	Concurrency int32    `json:"concurrency"`
	ActiveJobs  int32    `json:"active_jobs"`
	Draining    bool     `json:"draining"`
}
