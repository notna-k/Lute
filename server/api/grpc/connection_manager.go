package grpc

import (
	"errors"
	"sync"
	"time"

	pb "github.com/lute/agent/proto/agent"
)

var (
	ErrNoConnection = errors.New("no active connection for machine")
	ErrPingTimeout  = errors.New("heartbeat ping timed out")
)

// pingRequest is sent from the HeartbeatChecker to the stream handler goroutine.
// The handler writes a HeartbeatPing to the stream, waits for the pong, and
// sends the result back on resultCh.
type pingRequest struct {
	resultCh chan<- pingResult
}

type pingResult struct {
	Pong *pb.HeartbeatPong
	Err  error
}

// MachineConnection wraps a single bidirectional stream for one machine.
// All stream I/O happens inside the Run loop (single goroutine); the
// HeartbeatChecker communicates via the pingCh channel.
type MachineConnection struct {
	MachineID string
	stream    pb.AgentService_ConnectServer
	pingCh    chan pingRequest
}

func newMachineConnection(machineID string, stream pb.AgentService_ConnectServer) *MachineConnection {
	return &MachineConnection{
		MachineID: machineID,
		stream:    stream,
		pingCh:    make(chan pingRequest, 1),
	}
}

// Ping sends a HeartbeatPing over the stream and waits for the pong.
// Called by HeartbeatChecker from a different goroutine.
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

// Run processes ping requests and dispatches them over the stream.
// It blocks until the stream closes or the context is cancelled.
// Must be called from the gRPC Connect handler goroutine.
func (mc *MachineConnection) Run() {
	for {
		select {
		case <-mc.stream.Context().Done():
			return
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

			msg, err := mc.stream.Recv()
			if err != nil {
				req.resultCh <- pingResult{Err: err}
				return
			}
			req.resultCh <- pingResult{Pong: msg.GetHeartbeatPong()}
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
func (cm *ConnectionManager) Register(machineID string, stream pb.AgentService_ConnectServer) *MachineConnection {
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
