package worker

import (
	"context"
	"log/slog"
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
	workerRepo  *repos.WorkerRepository
	connMgr     *grpc.ConnectionManager
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
		workerRepo:  workerRepo,
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

func (h *HeartbeatChecker) Run(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	h.check(ctx)

	for {
		select {
		case <-ctx.Done():
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
		slog.Error("heartbeat: list monitored workers", "err", err)
		return
	}

	for _, w := range workers {
		workerID := w.ID.Hex()
		conn := h.connMgr.Get(workerID)

		if conn == nil {
			h.handleMiss(ctx, workerID)
			continue
		}

		slog.Debug("heartbeat: pinging worker", "worker_id", workerID)
		pong, err := conn.Ping(h.pingTimeout)
		if err != nil {
			slog.Warn("heartbeat: ping failed", "worker_id", workerID, "err", err)
			h.handleMiss(ctx, workerID)
			continue
		}

		var metrics map[string]interface{}
		if pong != nil {
			metrics = metricValueMapToInterface(pong.GetMetrics())
		}
		if err := h.workerRepo.UpdateHeartbeat(ctx, w.ID, metrics); err != nil {
			slog.Error("heartbeat: update", "worker_id", workerID, "err", err)
		} else {
			slog.Debug("heartbeat: worker ok", "worker_id", workerID)
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
		slog.Error("heartbeat: increment retry", "worker_id", workerID, "err", err)
		return
	}

	if newRetry >= h.maxRetries {
		if err := h.workerRepo.UpdateStatus(ctx, wid, "dead"); err != nil {
			slog.Error("heartbeat: mark dead", "worker_id", workerID, "err", err)
			return
		}
		slog.Warn("heartbeat: marked worker dead", "worker_id", workerID, "retries", newRetry, "max_retries", h.maxRetries)
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
