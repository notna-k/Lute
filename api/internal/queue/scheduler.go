package queue

import (
	"context"
	"log/slog"
	"time"
)

// Scheduler sweeps the queue each tick: it promotes due delayed jobs and reaps expired leases.
type Scheduler struct {
	engine   *Engine
	interval time.Duration
	// onJobsPromoted dispatches the queues that just received ready jobs.
	onJobsPromoted func(ctx context.Context, queueNames []string)
	// onLeasesExpired fails builds whose worker stopped reporting.
	onLeasesExpired func(ctx context.Context, leases []ExpiredLease)
}

func NewScheduler(
	engine *Engine,
	interval time.Duration,
	onJobsPromoted func(ctx context.Context, queueNames []string),
	onLeasesExpired func(ctx context.Context, leases []ExpiredLease),
) *Scheduler {
	if interval <= 0 {
		interval = time.Second
	}
	return &Scheduler{
		engine:          engine,
		interval:        interval,
		onJobsPromoted:  onJobsPromoted,
		onLeasesExpired: onLeasesExpired,
	}
}

// Run blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.promoteDelayed(ctx)
			s.reapExpiredLeases(ctx)
		}
	}
}

func (s *Scheduler) promoteDelayed(ctx context.Context) {
	promoted, queues, err := s.engine.PromoteDelayed(ctx)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("promote delayed jobs", "err", err)
		}
		return
	}
	if promoted == 0 {
		return
	}
	slog.Info("promoted delayed jobs", "count", promoted)
	if len(queues) > 0 {
		s.onJobsPromoted(ctx, queues)
	}
}

func (s *Scheduler) reapExpiredLeases(ctx context.Context) {
	leases, err := s.engine.ClaimExpiredLeases(ctx)
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("claim expired leases", "err", err)
		}
		return
	}
	if len(leases) == 0 {
		return
	}
	slog.Warn("reaping jobs whose worker stopped reporting", "count", len(leases))
	s.onLeasesExpired(ctx, leases)
}
