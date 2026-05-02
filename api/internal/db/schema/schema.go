package schema

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// DDL creates all application tables and indexes if they do not exist.
const DDL = `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	email TEXT NOT NULL,
	display_name TEXT NOT NULL,
	firebase_uid TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_firebase_uid ON users(firebase_uid);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

CREATE TABLE IF NOT EXISTS workers (
	id TEXT PRIMARY KEY,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	user_id TEXT,
	name TEXT NOT NULL,
	description TEXT NOT NULL,
	status TEXT NOT NULL,
	metadata TEXT,
	agent_ip TEXT,
	agent_version TEXT,
	last_seen INTEGER,
	metrics TEXT,
	heartbeat_retry INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_workers_user_agent_ip ON workers(user_id, agent_ip) WHERE agent_ip IS NOT NULL AND agent_ip != '';
CREATE INDEX IF NOT EXISTS idx_workers_user_id ON workers(user_id);

CREATE TABLE IF NOT EXISTS commands (
	id TEXT PRIMARY KEY,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	worker_id TEXT NOT NULL,
	command TEXT NOT NULL,
	args TEXT,
	env TEXT,
	status TEXT NOT NULL,
	output TEXT,
	exit_code INTEGER NOT NULL DEFAULT 0,
	error TEXT
);
CREATE INDEX IF NOT EXISTS idx_commands_worker_created ON commands(worker_id, created_at);

CREATE TABLE IF NOT EXISTS uptime_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	at INTEGER NOT NULL,
	alive INTEGER NOT NULL,
	dead INTEGER NOT NULL,
	total INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_uptime_snapshots_user_at ON uptime_snapshots(user_id, at);

CREATE TABLE IF NOT EXISTS worker_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	worker_id TEXT NOT NULL,
	at INTEGER NOT NULL,
	metrics TEXT
);
CREATE INDEX IF NOT EXISTS idx_worker_snapshots_worker_at ON worker_snapshots(worker_id, at);

CREATE TABLE IF NOT EXISTS job_executions (
	id TEXT PRIMARY KEY,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	job_id TEXT NOT NULL UNIQUE,
	worker_id TEXT NOT NULL,
	queue TEXT NOT NULL,
	type TEXT NOT NULL,
	success INTEGER NOT NULL,
	error TEXT,
	elapsed_ms INTEGER NOT NULL,
	log_file TEXT,
	execution_log_file TEXT,
	finished_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_job_executions_finished ON job_executions(finished_at);

CREATE TABLE IF NOT EXISTS api_keys (
	id TEXT PRIMARY KEY,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	user_id TEXT NOT NULL,
	name TEXT NOT NULL,
	prefix TEXT NOT NULL UNIQUE,
	hash TEXT NOT NULL,
	last_used_at INTEGER,
	revoked_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_created ON api_keys(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS runs (
	id TEXT PRIMARY KEY,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	job_id TEXT NOT NULL UNIQUE,
	user_id TEXT NOT NULL,
	api_key_id TEXT,
	queue TEXT NOT NULL,
	type TEXT NOT NULL,
	idempotency_key TEXT,
	webhook_url TEXT,
	webhook_secret TEXT,
	webhook_events TEXT
);
CREATE INDEX IF NOT EXISTS idx_runs_user_created ON runs(user_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_runs_user_idempotency ON runs(user_id, idempotency_key) WHERE idempotency_key IS NOT NULL AND idempotency_key != '';

CREATE TABLE IF NOT EXISTS webhook_deliveries (
	id TEXT PRIMARY KEY,
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL,
	run_id TEXT NOT NULL,
	job_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	event TEXT NOT NULL,
	url TEXT NOT NULL,
	payload BLOB NOT NULL,
	signature TEXT NOT NULL,
	signed_timestamp INTEGER NOT NULL,
	status TEXT NOT NULL,
	attempts INTEGER NOT NULL,
	max_attempts INTEGER NOT NULL,
	next_retry_at INTEGER NOT NULL,
	last_error TEXT,
	response_status INTEGER NOT NULL DEFAULT 0,
	delivered_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_retry ON webhook_deliveries(status, next_retry_at);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_job ON webhook_deliveries(job_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_user_created ON webhook_deliveries(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS queue_slots (
	job_id TEXT PRIMARY KEY NOT NULL,
	queue_name TEXT NOT NULL,
	payload TEXT NOT NULL,
	lane TEXT NOT NULL CHECK (lane IN ('ready','delayed','none')),
	priority REAL NOT NULL DEFAULT 0,
	release_at_ms INTEGER NOT NULL DEFAULT 0,
	updated_at_ms INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_queue_slots_ready ON queue_slots(queue_name, priority DESC, job_id ASC) WHERE lane = 'ready';
CREATE INDEX IF NOT EXISTS idx_queue_slots_delayed ON queue_slots(release_at_ms, job_id) WHERE lane = 'delayed';

CREATE TABLE IF NOT EXISTS queue_dlq (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	queue_name TEXT NOT NULL,
	job_id TEXT NOT NULL UNIQUE
);
CREATE INDEX IF NOT EXISTS idx_queue_dlq_queue_id ON queue_dlq(queue_name, id);

CREATE TABLE IF NOT EXISTS queue_stats_minute (
	queue_name TEXT NOT NULL,
	minute_bucket INTEGER NOT NULL,
	processed INTEGER NOT NULL DEFAULT 0,
	failed INTEGER NOT NULL DEFAULT 0,
	enqueued INTEGER NOT NULL DEFAULT 0,
	latency_sum INTEGER NOT NULL DEFAULT 0,
	latency_count INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (queue_name, minute_bucket)
);
`

// Apply runs DDL statements one at a time.
func Apply(ctx context.Context, db *sql.DB) error {
	stmts := strings.Split(DDL, ";")
	for _, s := range stmts {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		q := s + ";"
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("exec schema: %w: %s", err, q)
		}
	}
	return nil
}
