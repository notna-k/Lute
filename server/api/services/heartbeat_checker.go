package services

import (
	"context"
	"log"
	"time"

	luteGrpc "github.com/lute/api/grpc"
	"github.com/lute/api/repository"
)

// HeartbeatChecker periodically pings connected agents over their
// bidirectional gRPC streams. On a successful pong the retry counter is
// reset; on failure it is incremented. Once retries exceed max the machine
// is marked dead and no longer polled.
type HeartbeatChecker struct {
	machineRepo *repository.MachineRepository
	connMgr     *luteGrpc.ConnectionManager
	interval    time.Duration
	pingTimeout time.Duration
	maxRetries  int
}

func NewHeartbeatChecker(
	machineRepo *repository.MachineRepository,
	connMgr *luteGrpc.ConnectionManager,
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
	}
}

func (h *HeartbeatChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	log.Printf("Heartbeat checker started (interval %s, ping timeout %s, max retries %d)",
		h.interval, h.pingTimeout, h.maxRetries)

	for {
		select {
		case <-ctx.Done():
			log.Println("Heartbeat checker stopped")
			return
		case <-ticker.C:
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

		pong, err := conn.Ping(h.pingTimeout)
		if err != nil {
			log.Printf("Heartbeat checker: ping %s failed: %v", machineID, err)
			h.handleMiss(ctx, machineID)
			continue
		}

		var metrics map[string]string
		if pong != nil {
			metrics = pong.GetMetrics()
		}
		if err := h.machineRepo.UpdateHeartbeat(ctx, m.ID, metrics); err != nil {
			log.Printf("Heartbeat checker: update heartbeat %s: %v", machineID, err)
		}
	}
}

func (h *HeartbeatChecker) handleMiss(ctx context.Context, machineID string) {
	mid, err := luteGrpc.ParseMachineID(machineID)
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
