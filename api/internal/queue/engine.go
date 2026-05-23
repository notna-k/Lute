package queue

import (
	jobtypes "github.com/lute/api/internal/queuejob"

	"github.com/lute/api/internal/db/repos"
)

// Job mirrors the persisted JSON envelope stored in queue_slots.payload.
type Job = jobtypes.Job

// EnqueueOpts are enqueue-time controls (priority, delay, retries).
type EnqueueOpts = jobtypes.EnqueueOpts

// Engine delegates queue persistence exclusively to repositories (GORM).
type Engine struct {
	*repos.JobQueueRepository
}

// NewEngine returns a queue façade over JobQueueRepository.
func NewEngine(r *repos.JobQueueRepository) *Engine {
	return &Engine{JobQueueRepository: r}
}
