package queue

import (
	"context"
	"log"
	"time"
)

// Scheduler polls the delayed and cron ZSETs, promoting jobs whose
// run_at/next_run timestamp has passed back into their queue.
type Scheduler struct {
	engine        *Engine
	interval      time.Duration
	onJobsPromoted func(ctx context.Context, queueNames []string)
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
			promoted, queues, err := s.engine.PromoteDelayed(ctx)
			if err != nil {
				log.Printf("Scheduler: promote delayed: %v", err)
			} else if promoted > 0 {
				log.Printf("Scheduler: promoted %d delayed jobs", promoted)
				if s.onJobsPromoted != nil && len(queues) > 0 {
					s.onJobsPromoted(ctx, queues)
				}
			}
		}
	}
}
