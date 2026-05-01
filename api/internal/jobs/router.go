package jobs

import (
	"github.com/gin-gonic/gin"
)

// SetupRoutes sets up job queue, stats, DLQ, and executions routes.
func SetupRoutes(
	r *gin.RouterGroup,
	jobHandler *JobHandler,
	queueHandler *QueueHandler,
	dlqHandler *DLQHandler,
	executionsHandler *ExecutionsHandler,
) {
	r.GET("/executions", executionsHandler.ListExecutions)
	r.GET("/executions/filter-options", executionsHandler.ExecutionFilterOptions)

	r.POST("/jobs", jobHandler.Enqueue)
	r.GET("/jobs/:id/logs", jobHandler.GetJobLogs)
	r.GET("/jobs/:id", jobHandler.GetJob)
	r.POST("/jobs/:id/retry", jobHandler.RetryJob)
	r.DELETE("/jobs/:id", jobHandler.CancelJob)

	r.GET("/queues", queueHandler.ListQueues)
	r.GET("/queues/:name/jobs", queueHandler.ListQueueJobs)
	r.POST("/queues/:name/purge", queueHandler.PurgeQueue)

	r.GET("/stats/queues", queueHandler.GetAllStats)
	r.GET("/stats/queues/:name", queueHandler.GetStats)

	r.GET("/dlq/:queue", dlqHandler.ListDLQ)
	r.POST("/dlq/:queue/retry-all", dlqHandler.RetryAll)
}
