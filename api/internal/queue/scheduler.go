package queue

import (
	"context"
	"log"
	"time"

	"github.com/lute/api/internal/db/repos"
)

// ExpiredLease is a dispatched job whose worker stopped accounting for it.
type ExpiredLease = repos.ExpiredLease

// Scheduler sweeps the queue tables each tick: promote due delayed jobs, reap expired leases.
type Scheduler struct {
	engine          *Engine
	interval        time.Duration
	onJobsPromoted  func(ctx context.Context, queueNames []string)
	onLeasesExpired func(ctx context.Context, leases []ExpiredLease)
}

func NewScheduler(engine *Engine, interval time.Duration) *Scheduler {
	if interval == 0 {
		interval = time.Second
	}
	return &Scheduler{engine: engine, interval: interval}
}

// SetOnJobsPromoted registers a callback invoked after delayed jobs are moved
// back to their queues (e.g. to assign them to workers). Optional.
func (s *Scheduler) SetOnJobsPromoted(fn func(ctx context.Context, queueNames []string)) {
	s.onJobsPromoted = fn
}

// SetOnLeasesExpired registers the callback that fails claimed jobs; optional.
func (s *Scheduler) SetOnLeasesExpired(fn func(ctx context.Context, leases []ExpiredLease)) {
	s.onLeasesExpired = fn
}

// Run blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	log.Printf("Queue scheduler started (poll interval %s)", s.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Queue scheduler stopped")
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
		log.Printf("Scheduler: promote delayed: %v", err)
		return
	}
	if promoted == 0 {
		return
	}
	log.Printf("Scheduler: promoted %d delayed jobs", promoted)
	if s.onJobsPromoted != nil && len(queues) > 0 {
		s.onJobsPromoted(ctx, queues)
	}
}

// reapExpiredLeases hands builds whose worker never reported back to the callback to be failed.
func (s *Scheduler) reapExpiredLeases(ctx context.Context) {
	leases, err := s.engine.ClaimExpiredLeases(ctx)
	if err != nil {
		log.Printf("Scheduler: claim expired leases: %v", err)
		return
	}
	if len(leases) == 0 {
		return
	}
	log.Printf("Scheduler: reaping %d job(s) whose worker stopped reporting", len(leases))
	if s.onLeasesExpired != nil {
		s.onLeasesExpired(ctx, leases)
	}
}
