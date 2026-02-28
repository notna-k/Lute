package queue

import (
	"context"
	"log"
	"time"
)

// Scheduler polls the delayed and cron ZSETs, promoting jobs whose
// run_at/next_run timestamp has passed back into their queue.
type Scheduler struct {
	engine   *Engine
	interval time.Duration
}

func NewScheduler(engine *Engine, interval time.Duration) *Scheduler {
	if interval == 0 {
		interval = time.Second
	}
	return &Scheduler{engine: engine, interval: interval}
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
			promoted, err := s.engine.PromoteDelayed(ctx)
			if err != nil {
				log.Printf("Scheduler: promote delayed: %v", err)
			} else if promoted > 0 {
				log.Printf("Scheduler: promoted %d delayed jobs", promoted)
			}
		}
	}
}
