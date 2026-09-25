//go:build e2e

package harness

import "encoding/json"

// These types mirror core's wire format, not its internal models.

type Session struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	TokenType   string `json:"token_type"`
	User        User   `json:"user"`
}

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type ClaimCode struct {
	Code      string `json:"code"`
	ExpiresAt string `json:"expires_at"`
}

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

type Registered struct {
	WorkerID    string `json:"worker_id"`
	GRPCAddress string `json:"grpc_address"`
	Message     string `json:"message"`
}

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
	LastSeen     string            `json:"last_seen"`
}

type ConnectedWorker struct {
	WorkerID    string   `json:"worker_id"`
	Queues      []string `json:"queues"`
	Concurrency int32    `json:"concurrency"`
	ActiveJobs  int32    `json:"active_jobs"`
	Draining    bool     `json:"draining"`
}

type WorkerLiveStatus struct {
	WorkerID     string         `json:"worker_id"`
	Name         string         `json:"name"`
	Status       string         `json:"status"`
	AgentIP      string         `json:"agent_ip"`
	AgentVersion string         `json:"agent_version"`
	LastSeen     string         `json:"last_seen"`
	Metrics      map[string]any `json:"metrics"`
}

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

type Option struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
	Hint  string `json:"hint,omitempty"`
	Tone  string `json:"tone,omitempty"`
}

type JobSource struct {
	Repo   string `json:"repo"`
	Path   string `json:"path"`
	Commit string `json:"commit"`
}

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

type Build struct {
	ID          string            `json:"id"`
	RunID       string            `json:"runId"`
	JobID       string            `json:"jobId"`
	JobSlug     string            `json:"jobSlug"`
	Status      string            `json:"status"`
	Environment string            `json:"environment,omitempty"`
	StartedAt   int64             `json:"startedAt"`
	DurationMs  int64             `json:"durationMs,omitempty"`
	Params      map[string]string `json:"params,omitempty"`
	AdHoc       bool              `json:"adHoc,omitempty"`
}

type TriggerRequest struct {
	Values     map[string]any   `json:"values"`
	Parameters []ParameterField `json:"parameters,omitempty"`
}

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

type SyncResult struct {
	Added     int      `json:"added"`
	Updated   int      `json:"updated"`
	Unchanged int      `json:"unchanged"`
	Detached  int      `json:"detached"`
	Pruned    int      `json:"pruned"`
	Skipped   []string `json:"skipped"`
}

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

type Enqueued struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type LogPage struct {
	Lines      []string `json:"lines"`
	Direction  string   `json:"direction"`
	HasMore    bool     `json:"has_more"`
	FileSize   int64    `json:"file_size"`
	NextCursor string   `json:"next_cursor"`
	Error      string   `json:"error"`
}

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

type QueueInfo struct {
	Name  string `json:"name"`
	Depth int64  `json:"depth"`
}

type Settings struct {
	AllowAdhocBuilds bool `json:"allowAdhocBuilds"`
	PruneDefinitions bool `json:"pruneDefinitions"`
}

type SettingsUpdate struct {
	AllowAdhocBuilds *bool `json:"allowAdhocBuilds,omitempty"`
	PruneDefinitions *bool `json:"pruneDefinitions,omitempty"`
}

type APIKey struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	Token      string  `json:"token,omitempty"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	Revoked    bool    `json:"revoked"`
}

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

type WebhookSpec struct {
	URL    string   `json:"url"`
	Secret string   `json:"secret,omitempty"`
	Events []string `json:"events,omitempty"`
}

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

// ContainerSpec is the "container" job payload shared by core and the worker.
type ContainerSpec struct {
	SourceRepository string            `json:"source_repository,omitempty"`
	Runtime          string            `json:"runtime"`
	RequestParams    map[string]string `json:"request_params,omitempty"`
	Command          string            `json:"command"`
}
