package router

import (
	"github.com/gin-gonic/gin"

	"github.com/lute/api/handlers"
)

// SetupJobRoutes sets up job queue, stats, DLQ, and workers routes
func SetupJobRoutes(
	r *gin.RouterGroup,
	jobHandler *handlers.JobHandler,
	queueHandler *handlers.QueueHandler,
	dlqHandler *handlers.DLQHandler,
	workersHandler *handlers.WorkersInfoHandler,
) {
	// Jobs
	r.POST("/jobs", jobHandler.Enqueue)
	r.GET("/jobs/:id", jobHandler.GetJob)
	r.POST("/jobs/:id/retry", jobHandler.RetryJob)
	r.DELETE("/jobs/:id", jobHandler.CancelJob)

	// Queues
	r.GET("/queues", queueHandler.ListQueues)
	r.GET("/queues/:name/jobs", queueHandler.ListQueueJobs)
	r.POST("/queues/:name/purge", queueHandler.PurgeQueue)

	// Workers (job workers info)
	r.GET("/workers", workersHandler.ListWorkers)

	// Stats
	r.GET("/stats/queues", queueHandler.GetAllStats)
	r.GET("/stats/queues/:name", queueHandler.GetStats)

	// DLQ
	r.GET("/dlq/:queue", dlqHandler.ListDLQ)
	r.POST("/dlq/:queue/retry-all", dlqHandler.RetryAll)
}
