package models

import "github.com/lute/api/internal/db/enums"

// QueueSlot is the durable-backed queue envelope (serialized job JSON in Payload).
type QueueSlot struct {
	JobID       string          `json:"job_id" gorm:"column:job_id;primaryKey"`
	QueueName   string          `json:"queue_name" gorm:"column:queue_name;not null"`
	Payload     string          `json:"-" gorm:"column:payload;type:text;not null"`
	Lane        enums.QueueLane `json:"lane" gorm:"column:lane;type:varchar(16);not null"`
	Priority    float64         `json:"priority" gorm:"column:priority;default:0"`
	ReleaseAtMS int64           `json:"release_at_ms" gorm:"column:release_at_ms;default:0"`
	UpdatedAtMS int64           `json:"updated_at_ms" gorm:"column:updated_at_ms;not null"`
}

func (*QueueSlot) TableName() string { return "queue_slots" }

// QueueDLQ links dead-lettered job IDs to queues.
type QueueDLQ struct {
	ID        uint64 `gorm:"primaryKey;autoIncrement"`
	QueueName string `json:"queue_name" gorm:"column:queue_name;not null;index:idx_queue_dlq_queue_id"`
	JobID     string `json:"job_id" gorm:"column:job_id;not null;uniqueIndex"`
}

func (*QueueDLQ) TableName() string { return "queue_dlq" }

// QueueStatsMinute holds rollup counters keyed by bucket.
type QueueStatsMinute struct {
	QueueName    string `gorm:"column:queue_name;primaryKey"`
	MinuteBucket int64  `gorm:"column:minute_bucket;primaryKey"`
	Processed    int64  `gorm:"column:processed;default:0"`
	Failed       int64  `gorm:"column:failed;default:0"`
	Enqueued     int64  `gorm:"column:enqueued;default:0"`
	LatencySum   int64  `gorm:"column:latency_sum;default:0"`
	LatencyCount int64  `gorm:"column:latency_count;default:0"`
}

func (*QueueStatsMinute) TableName() string { return "queue_stats_minute" }
