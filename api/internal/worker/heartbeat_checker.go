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
// reset; on failure it is incremented. Once retries exceed max the worker
// is marked dead and no longer polled.
type HeartbeatChecker struct {
	workerRepo *repos.WorkerRepository
	connMgr    *grpc.ConnectionManager
	interval    time.Duration
	pingTimeout time.Duration
	maxRetries  int
	runNow      chan struct{}
}

func NewHeartbeatChecker(
	workerRepo *repos.WorkerRepository,
	connMgr *grpc.ConnectionManager,
	interval time.Duration,
	pingTimeout time.Duration,
	maxRetries int,
) *HeartbeatChecker {
	return &HeartbeatChecker{
		workerRepo: workerRepo,
		connMgr:    connMgr,
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
	workers, err := h.workerRepo.ListMonitored(ctx)
	if err != nil {
		log.Printf("Heartbeat checker: list monitored: %v", err)
		return
	}

	for _, w := range workers {
		workerID := w.ID.Hex()
		conn := h.connMgr.Get(workerID)

		if conn == nil {
			h.handleMiss(ctx, workerID)
			continue
		}

		log.Printf("Heartbeat checker: pinging worker %s", workerID)
		pong, err := conn.Ping(h.pingTimeout)
		if err != nil {
			log.Printf("Heartbeat checker: ping %s failed: %v", workerID, err)
			h.handleMiss(ctx, workerID)
			continue
		}

		var metrics map[string]interface{}
		if pong != nil {
			metrics = metricValueMapToInterface(pong.GetMetrics())
		}
		if err := h.workerRepo.UpdateHeartbeat(ctx, w.ID, metrics); err != nil {
			log.Printf("Heartbeat checker: update heartbeat %s: %v", workerID, err)
		} else {
			log.Printf("Heartbeat checker: worker %s OK", workerID)
		}
	}
}

func (h *HeartbeatChecker) handleMiss(ctx context.Context, workerID string) {
	wid, err := grpc.ParseWorkerID(workerID)
	if err != nil {
		return
	}

	newRetry, err := h.workerRepo.IncrementHeartbeatRetry(ctx, wid)
	if err != nil {
		log.Printf("Heartbeat checker: increment retry %s: %v", workerID, err)
		return
	}

	if newRetry >= h.maxRetries {
		if err := h.workerRepo.UpdateStatus(ctx, wid, "dead"); err != nil {
			log.Printf("Heartbeat checker: mark dead %s: %v", workerID, err)
			return
		}
		log.Printf("Heartbeat checker: marked %s as dead (retry %d >= %d)", workerID, newRetry, h.maxRetries)
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
