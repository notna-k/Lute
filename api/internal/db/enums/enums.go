package enums

// Portable "enums": varchar columns + typed string aliases (no native PG ENUM types).

type WorkerStatus = string

const (
	WorkerPending    WorkerStatus = "pending"
	WorkerRegistered WorkerStatus = "registered"
	WorkerAlive      WorkerStatus = "alive"
	WorkerDead       WorkerStatus = "dead"
)

type CommandStatus = string

const (
	CommandPending   CommandStatus = "pending"
	CommandRunning   CommandStatus = "running"
	CommandCompleted CommandStatus = "completed"
	CommandFailed    CommandStatus = "failed"
)

type WebhookDeliveryStatus = string

const (
	WebhookDeliveryPending   WebhookDeliveryStatus = "pending"
	WebhookDeliveryInFlight  WebhookDeliveryStatus = "in_flight"
	WebhookDeliveryDelivered WebhookDeliveryStatus = "delivered"
	WebhookDeliveryFailed    WebhookDeliveryStatus = "failed"
)

// QueueLane is persisted for queue_slots.
type QueueLane = string

const (
	QueueLaneReady   QueueLane = "ready"
	QueueLaneDelayed QueueLane = "delayed"
	QueueLaneNone    QueueLane = "none"
)

// QueueJobStatus is the JSON job envelope status stored inside queue_slots.payload.
type QueueJobStatus = string

const (
	QueueJobPending QueueJobStatus = "pending"
	QueueJobRunning QueueJobStatus = "running"
	QueueJobDone    QueueJobStatus = "done"
	QueueJobDead    QueueJobStatus = "dead"
)
