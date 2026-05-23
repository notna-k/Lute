package migrate

import "gorm.io/gorm"

// ApplySecondaryIndexes creates partial indexes and composite indexes shared by SQLite and PostgreSQL.
//
// DDL lives here—not in repositories—because GORM struct tags cannot express every partial UNIQUE.
func ApplySecondaryIndexes(db *gorm.DB) error {
	stmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_firebase_uid_nonempty ON users(firebase_uid) WHERE firebase_uid IS NOT NULL AND firebase_uid != ''`,
		`CREATE INDEX IF NOT EXISTS idx_workers_user_id ON workers(user_id)`,
		// workers: one logical agent per IP per user when IP is set
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_workers_user_agent_ip ON workers(user_id, agent_ip) WHERE agent_ip IS NOT NULL AND agent_ip != ''`,
		// runs: idempotency per user when key is set
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_runs_user_idempotency ON runs(user_id, idempotency_key) WHERE idempotency_key IS NOT NULL AND idempotency_key != ''`,
		`CREATE INDEX IF NOT EXISTS idx_runs_user_created ON runs(user_id, created_at DESC)`,

		`CREATE INDEX IF NOT EXISTS idx_commands_worker_created ON commands(worker_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_api_keys_user_created ON api_keys(user_id, created_at DESC)`,

		`CREATE INDEX IF NOT EXISTS idx_worker_snapshots_worker_at ON worker_snapshots(worker_id, at)`,
		`CREATE INDEX IF NOT EXISTS idx_uptime_snapshots_user_at ON uptime_snapshots(user_id, at)`,
		`CREATE INDEX IF NOT EXISTS idx_job_executions_finished ON job_executions(finished_at)`,

		`CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_retry ON webhook_deliveries(status, next_retry_at)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_job ON webhook_deliveries(job_id)`,
		`CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_user_created ON webhook_deliveries(user_id, created_at DESC)`,

		`CREATE INDEX IF NOT EXISTS idx_queue_slots_ready ON queue_slots(queue_name, priority DESC, job_id ASC) WHERE lane = 'ready'`,
		`CREATE INDEX IF NOT EXISTS idx_queue_slots_delayed ON queue_slots(release_at_ms, job_id) WHERE lane = 'delayed'`,
		`CREATE INDEX IF NOT EXISTS idx_queue_dlq_queue_id ON queue_dlq(queue_name, id)`,
	}
	for _, q := range stmts {
		if err := db.Exec(q).Error; err != nil {
			return err
		}
	}
	return nil
}
