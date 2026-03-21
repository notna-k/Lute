package worker

import (
	"context"
	"log"
	"time"

	pb "github.com/lute/proto"

	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/grpc"
)

// HeartbeatChecker periodically pings connected agents over their
// bidirectional gRPC streams. On a successful pong the retry counter is
// reset; on failure it is incremented. Once retries exceed max the machine
// is marked dead and no longer polled.
type HeartbeatChecker struct {
	machineRepo *repos.MachineRepository
	connMgr     *grpc.ConnectionManager
	interval    time.Duration
	pingTimeout time.Duration
	maxRetries  int
	runNow      chan struct{}
}

func NewHeartbeatChecker(
	machineRepo *repos.MachineRepository,
	connMgr *grpc.ConnectionManager,
	interval time.Duration,
	pingTimeout time.Duration,
	maxRetries int,
) *HeartbeatChecker {
	return &HeartbeatChecker{
		machineRepo: machineRepo,
		connMgr:     connMgr,
		interval:    interval,
		pingTimeout: pingTimeout,
		maxRetries:  maxRetries,
		runNow:      make(chan struct{}, 1),
	}
}

func (h *HeartbeatChecker) TriggerCheck() {
	select {
	case h.runNow <- struct{}{}:
	default:
	}
}

func (h *HeartbeatChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	log.Printf("Heartbeat checker started (interval %s, ping timeout %s, max retries %d)",
		h.interval, h.pingTimeout, h.maxRetries)

	h.check(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Heartbeat checker stopped")
			return
		case <-ticker.C:
			h.check(ctx)
		case <-h.runNow:
			h.check(ctx)
		}
	}
}

func (h *HeartbeatChecker) check(ctx context.Context) {
	machines, err := h.machineRepo.ListMonitored(ctx)
	if err != nil {
		log.Printf("Heartbeat checker: list monitored: %v", err)
		return
	}

	for _, m := range machines {
		machineID := m.ID.Hex()
		conn := h.connMgr.Get(machineID)

		if conn == nil {
			h.handleMiss(ctx, machineID)
			continue
		}

		log.Printf("Heartbeat checker: pinging machine %s", machineID)
		pong, err := conn.Ping(h.pingTimeout)
		if err != nil {
			log.Printf("Heartbeat checker: ping %s failed: %v", machineID, err)
			h.handleMiss(ctx, machineID)
			continue
		}

		var metrics map[string]interface{}
		if pong != nil {
			metrics = metricValueMapToInterface(pong.GetMetrics())
		}
		if err := h.machineRepo.UpdateHeartbeat(ctx, m.ID, metrics); err != nil {
			log.Printf("Heartbeat checker: update heartbeat %s: %v", machineID, err)
		} else {
			log.Printf("Heartbeat checker: machine %s OK", machineID)
		}
	}
}

func (h *HeartbeatChecker) handleMiss(ctx context.Context, machineID string) {
	mid, err := grpc.ParseMachineID(machineID)
	if err != nil {
		return
	}

	newRetry, err := h.machineRepo.IncrementHeartbeatRetry(ctx, mid)
	if err != nil {
		log.Printf("Heartbeat checker: increment retry %s: %v", machineID, err)
		return
	}

	if newRetry >= h.maxRetries {
		if err := h.machineRepo.UpdateStatus(ctx, mid, "dead"); err != nil {
			log.Printf("Heartbeat checker: mark dead %s: %v", machineID, err)
			return
		}
		log.Printf("Heartbeat checker: marked %s as dead (retry %d >= %d)", machineID, newRetry, h.maxRetries)
	}
}

var canonicalMetricKeys = map[string]bool{
	"cpu_load": true, "mem_usage_mb": true, "disk_used_gb": true, "disk_total_gb": true,
}

func metricValueMapToInterface(proto map[string]*pb.MetricValue) map[string]interface{} {
	if len(proto) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(canonicalMetricKeys))
	for k, mv := range proto {
		if !canonicalMetricKeys[k] || mv == nil {
			continue
		}
		switch v := mv.Kind.(type) {
		case *pb.MetricValue_I:
			out[k] = v.I
		case *pb.MetricValue_F:
			out[k] = v.F
		case *pb.MetricValue_S:
			out[k] = v.S
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
