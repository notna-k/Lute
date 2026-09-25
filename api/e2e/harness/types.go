//go:build e2e

package harness

import "encoding/json"

// The types here mirror what core puts on the wire, not its internal models. A test
// asserting on these is asserting on the contract the panel and the public API see.

// Session is the result of signing in.
type Session struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	TokenType   string `json:"token_type"`
	User        User   `json:"user"`
}

// User is the panel's view of an account.
type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

// ClaimCode is a single-use code that links a new agent to the issuing account.
type ClaimCode struct {
	Code      string `json:"code"`
	ExpiresAt string `json:"expires_at"`
}

// WorkerRegistration is what an agent sends to claim itself.
type WorkerRegistration struct {
	Name      string            `json:"name"`
	Hostname  string            `json:"hostname,omitempty"`
	OS        string            `json:"os,omitempty"`
	Arch      string            `json:"arch,omitempty"`
	CPUs      int               `json:"cpus,omitempty"`
	IP        string            `json:"ip,omitempty"`
	Version   string            `json:"version,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	ClaimCode string            `json:"claim_code,omitempty"`
}

// Registered is core's answer to a successful claim.
type Registered struct {
	WorkerID    string `json:"worker_id"`
	GRPCAddress string `json:"grpc_address"`
	Message     string `json:"message"`
}

// Worker is a registered build host.
type Worker struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Status       string            `json:"status"`
	AgentIP      string            `json:"agent_ip"`
	AgentVersion string            `json:"agent_version"`
	Labels       map[string]string `json:"labels"`
	Metrics      map[string]any    `json:"metrics"`
	// LastSeen is an RFC3339 timestamp, empty until the host has reported in.
	LastSeen string `json:"last_seen"`
}

// ConnectedWorker is a live gRPC stream as core sees it.
type ConnectedWorker struct {
	WorkerID    string   `json:"worker_id"`
	Queues      []string `json:"queues"`
	Concurrency int32    `json:"concurrency"`
	ActiveJobs  int32    `json:"active_jobs"`
	Draining    bool     `json:"draining"`
}

// WorkerLiveStatus is the status endpoint's answer, which omits agent detail until
// the worker has actually reported in.
type WorkerLiveStatus struct {
	WorkerID     string         `json:"worker_id"`
	Name         string         `json:"name"`
	Status       string         `json:"status"`
	AgentIP      string         `json:"agent_ip"`
	AgentVersion string         `json:"agent_version"`
	LastSeen     string         `json:"last_seen"`
	Metrics      map[string]any `json:"metrics"`
}

// ParameterField is one input in a definition's schema.
type ParameterField struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Label       string   `json:"label,omitempty"`
	Description string   `json:"description,omitempty"`
	EnvVar      string   `json:"envVar,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Default     any      `json:"default,omitempty"`
	Options     []Option `json:"options,omitempty"`
	SecretRef   string   `json:"secretRef,omitempty"`
}

// Option is a choice offered by a select or multiselect parameter.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
	Hint  string `json:"hint,omitempty"`
	Tone  string `json:"tone,omitempty"`
}

// JobSource records where a definition came from.
type JobSource struct {
	Repo   string `json:"repo"`
	Path   string `json:"path"`
	Commit string `json:"commit"`
}

// JobDefinition is the panel's view of a job definition.
type JobDefinition struct {
	Slug             string            `json:"slug"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Queue            string            `json:"queue"`
	LabelSelector    map[string]string `json:"labelSelector"`
	Runtime          string            `json:"runtime"`
	Command          string            `json:"command"`
	Source           JobSource         `json:"source"`
	Parameters       []ParameterField  `json:"parameters"`
	GitState         string            `json:"gitState"`
	SuccessRate      float64           `json:"successRate"`
	MedianDurationMs int64             `json:"medianDurationMs"`
	LastBuild        *Build            `json:"lastBuild,omitempty"`
	Recent           []string          `json:"recent,omitempty"`
}

// Build is one execution of a definition.
type Build struct {
	ID    string `json:"id"`
	RunID string `json:"runId"`
	// JobID addresses the build in the queue and log APIs. A run id and a job id
	// are different values, and mixing them up is a 404.
	JobID       string            `json:"jobId"`
	JobSlug     string            `json:"jobSlug"`
	Status      string            `json:"status"`
	Environment string            `json:"environment,omitempty"`
	StartedAt   int64             `json:"startedAt"`
	DurationMs  int64             `json:"durationMs,omitempty"`
	Params      map[string]string `json:"params,omitempty"`
	AdHoc       bool              `json:"adHoc,omitempty"`
}

// TriggerRequest starts a build. Parameters is only sent when the panel rendered a
// schema of its own, which is what makes the build ad-hoc.
type TriggerRequest struct {
	Values     map[string]any   `json:"values"`
	Parameters []ParameterField `json:"parameters,omitempty"`
}

// JobDefinitionRequest is a panel-authored template.
type JobDefinitionRequest struct {
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Queue         string            `json:"queue,omitempty"`
	Runtime       string            `json:"runtime"`
	Command       string            `json:"command"`
	SourceRepo    string            `json:"sourceRepo,omitempty"`
	LabelSelector map[string]string `json:"labelSelector,omitempty"`
	Parameters    []ParameterField  `json:"parameters,omitempty"`
}

// SyncResult is what a sync from the definition directory reports.
type SyncResult struct {
	Added     int      `json:"added"`
	Updated   int      `json:"updated"`
	Unchanged int      `json:"unchanged"`
	Detached  int      `json:"detached"`
	Pruned    int      `json:"pruned"`
	Skipped   []string `json:"skipped"`
}

// Job is the queue's view of a unit of work.
type Job struct {
	ID         string            `json:"id"`
	Queue      string            `json:"queue"`
	Type       string            `json:"type"`
	Payload    json.RawMessage   `json:"payload"`
	Status     string            `json:"status"`
	Attempts   int               `json:"attempts"`
	MaxRetries int               `json:"max_retries"`
	TimeoutSec int               `json:"timeout_sec"`
	Error      string            `json:"error,omitempty"`
	WorkerID   string            `json:"worker_id,omitempty"`
	EnqueuedAt int64             `json:"enqueued_at"`
	StartedAt  int64             `json:"started_at,omitempty"`
	DoneAt     int64             `json:"done_at,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
	Selector   map[string]string `json:"selector,omitempty"`
}

// EnqueueRequest puts a job straight on a queue, bypassing definitions.
type EnqueueRequest struct {
	Queue      string            `json:"queue"`
	Type       string            `json:"type"`
	Payload    json.RawMessage   `json:"payload,omitempty"`
	Priority   float64           `json:"priority,omitempty"`
	DelayMs    int64             `json:"delay_ms,omitempty"`
	MaxRetries int               `json:"max_retries,omitempty"`
	TimeoutSec int               `json:"timeout_sec,omitempty"`
	Selector   map[string]string `json:"selector,omitempty"`
}

// Enqueued identifies a freshly queued job.
type Enqueued struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// LogPage is a chunk of a build's log, read off the worker that ran it.
type LogPage struct {
	Lines      []string `json:"lines"`
	Direction  string   `json:"direction"`
	HasMore    bool     `json:"has_more"`
	FileSize   int64    `json:"file_size"`
	NextCursor string   `json:"next_cursor"`
	Error      string   `json:"error"`
}

// Execution is the record of a finished attempt.
type Execution struct {
	JobID            string `json:"job_id"`
	WorkerID         string `json:"worker_id"`
	Queue            string `json:"queue"`
	Type             string `json:"type"`
	Success          bool   `json:"success"`
	Error            string `json:"error"`
	ElapsedMs        int64  `json:"elapsed_ms"`
	LogFile          string `json:"log_file"`
	ExecutionLogFile string `json:"execution_log_file"`
}

// QueueInfo is one queue and how much is waiting on it.
type QueueInfo struct {
	Name  string `json:"name"`
	Depth int64  `json:"depth"`
}

// Settings are the operator policy switches.
type Settings struct {
	AllowAdhocBuilds bool `json:"allowAdhocBuilds"`
	PruneDefinitions bool `json:"pruneDefinitions"`
}

// SettingsUpdate leaves omitted fields unchanged.
type SettingsUpdate struct {
	AllowAdhocBuilds *bool `json:"allowAdhocBuilds,omitempty"`
	PruneDefinitions *bool `json:"pruneDefinitions,omitempty"`
}

// APIKey is a key as returned on creation — the token is shown once and never again.
type APIKey struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	Token      string  `json:"token,omitempty"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	Revoked    bool    `json:"revoked"`
}

// CreateRunRequest is the public API's way to start work.
type CreateRunRequest struct {
	Queue          string            `json:"queue"`
	Type           string            `json:"type"`
	Payload        json.RawMessage   `json:"payload,omitempty"`
	Priority       float64           `json:"priority,omitempty"`
	DelayMs        int64             `json:"delay_ms,omitempty"`
	MaxRetries     int               `json:"max_retries,omitempty"`
	TimeoutSec     int               `json:"timeout_sec,omitempty"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	Webhook        *WebhookSpec      `json:"webhook,omitempty"`
	Selector       map[string]string `json:"selector,omitempty"`
}

// WebhookSpec subscribes a run's events to an endpoint.
type WebhookSpec struct {
	URL    string   `json:"url"`
	Secret string   `json:"secret,omitempty"`
	Events []string `json:"events,omitempty"`
}

// Run is the public API's view of one unit of work.
type Run struct {
	ID            string   `json:"id"`
	Queue         string   `json:"queue"`
	Type          string   `json:"type"`
	Status        string   `json:"status"`
	Attempts      int      `json:"attempts"`
	MaxRetries    int      `json:"max_retries"`
	TimeoutSec    int      `json:"timeout_sec"`
	Error         string   `json:"error,omitempty"`
	WorkerID      string   `json:"worker_id,omitempty"`
	WebhookURL    string   `json:"webhook_url,omitempty"`
	WebhookEvents []string `json:"webhook_events,omitempty"`
	ElapsedMs     int64    `json:"elapsed_ms,omitempty"`
	WebhookSecret string   `json:"webhook_secret,omitempty"`
}

// ContainerSpec is the payload a "container" job carries. Both core and the worker
// speak this shape; a test asserting on it is asserting on their contract.
type ContainerSpec struct {
	SourceRepository string            `json:"source_repository,omitempty"`
	Runtime          string            `json:"runtime"`
	RequestParams    map[string]string `json:"request_params,omitempty"`
	Command          string            `json:"command"`
}
