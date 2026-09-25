//go:build e2e

package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Client talks to core the way the panel and the public API's consumers do: over
// HTTP, with a token or an API key, and nothing else. Tests use it so they assert on
// the contract rather than on core's internals.
type Client struct {
	t       *testing.T
	BaseURL string
	http    *http.Client

	accessToken string
	apiKey      string
}

// NewClient returns an unauthenticated client holding its own cookie jar, so the
// refresh cookie behaves exactly as it does in a browser.
func NewClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	return &Client{
		t:       t,
		BaseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Jar: jar, Timeout: 60 * time.Second},
	}
}

// WithAPIKey returns a copy of the client authenticating with an API key instead of
// a session token.
func (c *Client) WithAPIKey(token string) *Client {
	clone := *c
	clone.apiKey = token
	clone.accessToken = ""
	return &clone
}

// Token is the access token this client sends, for callers that need it elsewhere
// (the WebSocket handshake, for instance).
func (c *Client) Token() string { return c.accessToken }

// APIError is a non-2xx answer, decoded far enough to assert on.
type APIError struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]string
	Body    string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("HTTP %d (%s): %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.Message)
}

// StatusOf returns the HTTP status an error carries, or 0 if it is not an APIError.
func StatusOf(err error) int {
	var apiErr *APIError
	if err != nil && errors.As(err, &apiErr) {
		return apiErr.Status
	}
	return 0
}

// CodeOf returns the machine-readable error code, or "".
func CodeOf(err error) string {
	var apiErr *APIError
	if err != nil && errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return ""
}

// FieldsOf returns per-input validation detail, or nil.
func FieldsOf(err error) map[string]string {
	var apiErr *APIError
	if err != nil && errors.As(err, &apiErr) {
		return apiErr.Fields
	}
	return nil
}

// ── plumbing ──────────────────────────────────────────────────────────────────

func (c *Client) request(method, path string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshal %s %s: %w", method, path, err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch {
	case c.apiKey != "":
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	case c.accessToken != "":
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, raw, decodeAPIError(resp.StatusCode, raw)
	}
	return resp.StatusCode, raw, nil
}

func decodeAPIError(status int, raw []byte) error {
	var envelope struct {
		Error  any               `json:"error"`
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	}
	apiErr := &APIError{Status: status, Body: string(raw)}
	if err := json.Unmarshal(raw, &envelope); err == nil {
		apiErr.Code = envelope.Code
		apiErr.Fields = envelope.Fields
		switch msg := envelope.Error.(type) {
		case string:
			apiErr.Message = msg
		case map[string]any:
			// The public API nests {"error": {"code": ..., "message": ...}}.
			if s, ok := msg["message"].(string); ok {
				apiErr.Message = s
			}
			if s, ok := msg["code"].(string); ok && apiErr.Code == "" {
				apiErr.Code = s
			}
		}
	}
	if apiErr.Message == "" {
		apiErr.Message = strings.TrimSpace(string(raw))
	}
	return apiErr
}

// call sends a request and decodes a successful body into T.
func call[T any](c *Client, method, path string, body any) (T, error) {
	var out T
	_, raw, err := c.request(method, path, body)
	if err != nil {
		return out, err
	}
	if len(raw) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode %s %s: %w: %s", method, path, err, truncate(raw))
	}
	return out, nil
}

// callField decodes one named field out of an envelope response.
func callField[T any](c *Client, method, path, field string, body any) (T, error) {
	envelope, err := call[map[string]json.RawMessage](c, method, path, body)
	var out T
	if err != nil {
		return out, err
	}
	raw, ok := envelope[field]
	if !ok {
		return out, fmt.Errorf("%s %s: no %q field in response", method, path, field)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode %s of %s %s: %w", field, method, path, err)
	}
	return out, nil
}

func truncate(raw []byte) string {
	const max = 400
	if len(raw) <= max {
		return string(raw)
	}
	return string(raw[:max]) + "…"
}

// ── auth ──────────────────────────────────────────────────────────────────────

// Login signs in and keeps the access token and refresh cookie for later calls.
func (c *Client) Login(email, password string) (Session, error) {
	s, err := call[Session](c, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": email, "password": password})
	if err == nil {
		c.accessToken = s.AccessToken
	}
	return s, err
}

// Refresh exchanges the refresh cookie for a new session.
func (c *Client) Refresh() (Session, error) {
	s, err := call[Session](c, http.MethodPost, "/api/v1/auth/refresh", nil)
	if err == nil {
		c.accessToken = s.AccessToken
	}
	return s, err
}

// Logout revokes the session behind the refresh cookie.
func (c *Client) Logout() error {
	_, _, err := c.request(http.MethodPost, "/api/v1/auth/logout", nil)
	return err
}

// Me returns the signed-in user.
func (c *Client) Me() (User, error) {
	return call[User](c, http.MethodGet, "/api/v1/auth/me", nil)
}

// RefreshCookie returns the refresh cookie currently held, for tests that need to
// prove a rotated one stops working.
func (c *Client) RefreshCookie() string {
	u, err := url.Parse(c.BaseURL + "/api/v1/auth")
	if err != nil {
		return ""
	}
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == "lute_refresh" {
			return ck.Value
		}
	}
	return ""
}

// SetRefreshCookie forces the refresh cookie, so a test can replay an old one.
func (c *Client) SetRefreshCookie(value string) {
	u, err := url.Parse(c.BaseURL + "/api/v1/auth")
	if err != nil {
		return
	}
	c.http.Jar.SetCookies(u, []*http.Cookie{{Name: "lute_refresh", Value: value, Path: "/api/v1/auth"}})
}

// ── workers ───────────────────────────────────────────────────────────────────

// CreateClaimCode issues a single-use code for claiming a new agent.
func (c *Client) CreateClaimCode() (ClaimCode, error) {
	return call[ClaimCode](c, http.MethodPost, "/api/v1/workers/claim-code", nil)
}

// RegisterWorker is the unauthenticated bootstrap call an agent makes with its code.
func (c *Client) RegisterWorker(req WorkerRegistration) (Registered, error) {
	return call[Registered](c, http.MethodPost, "/api/public/v1/workers/bootstrap/register", req)
}

// ListWorkers returns the caller's workers, optionally filtered by labels.
func (c *Client) ListWorkers(labelFilters ...string) ([]Worker, error) {
	path := "/api/v1/workers"
	if len(labelFilters) > 0 {
		q := url.Values{}
		for _, f := range labelFilters {
			q.Add("label", f)
		}
		path += "?" + q.Encode()
	}
	return call[[]Worker](c, http.MethodGet, path, nil)
}

// GetWorker returns one worker.
func (c *Client) GetWorker(id string) (Worker, error) {
	return call[Worker](c, http.MethodGet, "/api/v1/workers/"+id, nil)
}

// WorkerStatus returns the live status view, which hides agent detail until the
// worker has reported in.
func (c *Client) WorkerStatus(id string) (WorkerLiveStatus, error) {
	return call[WorkerLiveStatus](c, http.MethodGet, "/api/v1/workers/"+id+"/status", nil)
}

// ConnectedWorkers lists the agents holding a live stream to core.
func (c *Client) ConnectedWorkers() ([]ConnectedWorker, error) {
	return callField[[]ConnectedWorker](c, http.MethodGet, "/api/v1/workers/connected", "workers", nil)
}

// PatchLabels replaces a worker's whole label set.
func (c *Client) PatchLabels(id string, labels map[string]string) (Worker, error) {
	return call[Worker](c, http.MethodPatch, "/api/v1/workers/"+id+"/labels",
		map[string]any{"labels": labels})
}

// DeleteWorker removes a worker, asking a live agent to stop.
func (c *Client) DeleteWorker(id string) error {
	_, _, err := c.request(http.MethodDelete, "/api/v1/workers/"+id, nil)
	return err
}

// ReEnableWorker moves a dead worker back to pending so its agent may reconnect.
func (c *Client) ReEnableWorker(id string) (Worker, error) {
	return call[Worker](c, http.MethodPost, "/api/v1/workers/"+id+"/re-enable", nil)
}

// ── job definitions and builds ────────────────────────────────────────────────

// ListJobDefs returns every definition with its build history.
func (c *Client) ListJobDefs() ([]JobDefinition, error) {
	return callField[[]JobDefinition](c, http.MethodGet, "/api/v1/job-definitions", "jobs", nil)
}

// GetJobDef returns one definition.
func (c *Client) GetJobDef(slug string) (JobDefinition, error) {
	return call[JobDefinition](c, http.MethodGet, "/api/v1/job-definitions/"+slug, nil)
}

// CreateJobDef saves a panel-authored template.
func (c *Client) CreateJobDef(req JobDefinitionRequest) (JobDefinition, error) {
	return call[JobDefinition](c, http.MethodPost, "/api/v1/job-definitions", req)
}

// UpdateJobDef rewrites a definition's spec, which makes a Git-synced one drift.
func (c *Client) UpdateJobDef(slug string, req JobDefinitionRequest) (JobDefinition, error) {
	return call[JobDefinition](c, http.MethodPut, "/api/v1/job-definitions/"+slug, req)
}

// SyncJobDefs re-reads the definition directory.
func (c *Client) SyncJobDefs() (SyncResult, error) {
	return call[SyncResult](c, http.MethodPost, "/api/v1/job-definitions/sync", nil)
}

// RevertJobDef restores a drifted definition to its Git snapshot.
func (c *Client) RevertJobDef(slug string) (JobDefinition, error) {
	return call[JobDefinition](c, http.MethodPost, "/api/v1/job-definitions/"+slug+"/revert", nil)
}

// ExportJobDef returns one definition as the YAML a commit would carry.
func (c *Client) ExportJobDef(slug string) (string, error) {
	_, raw, err := c.request(http.MethodGet, "/api/v1/job-definitions/"+slug+"/yaml", nil)
	return string(raw), err
}

// ExportJobDefsZip returns the export archive.
func (c *Client) ExportJobDefsZip() ([]byte, error) {
	_, raw, err := c.request(http.MethodGet, "/api/v1/job-definitions/export.zip", nil)
	return raw, err
}

// Trigger starts a build of a definition with the given parameter values.
func (c *Client) Trigger(slug string, values map[string]any) (Build, error) {
	return call[Build](c, http.MethodPost, "/api/v1/job-definitions/"+slug+"/trigger",
		TriggerRequest{Values: values})
}

// TriggerWithSchema starts a build against a schema the caller supplies, which is
// what makes a build ad-hoc when it differs from the committed definition.
func (c *Client) TriggerWithSchema(slug string, values map[string]any, params []ParameterField) (Build, error) {
	return call[Build](c, http.MethodPost, "/api/v1/job-definitions/"+slug+"/trigger",
		TriggerRequest{Values: values, Parameters: params})
}

// Builds lists a definition's recent builds.
func (c *Client) Builds(slug string) ([]Build, error) {
	return callField[[]Build](c, http.MethodGet, "/api/v1/job-definitions/"+slug+"/builds", "builds", nil)
}

// ── queue ─────────────────────────────────────────────────────────────────────

// Enqueue puts a job on a queue directly.
func (c *Client) Enqueue(req EnqueueRequest) (Enqueued, error) {
	return call[Enqueued](c, http.MethodPost, "/api/v1/jobs", req)
}

// GetJob returns the queue's view of a job.
func (c *Client) GetJob(id string) (Job, error) {
	return call[Job](c, http.MethodGet, "/api/v1/jobs/"+id, nil)
}

// CancelJob drops a job that has not been dispatched.
func (c *Client) CancelJob(id string) error {
	_, _, err := c.request(http.MethodDelete, "/api/v1/jobs/"+id, nil)
	return err
}

// LogOptions selects which part of a build's log to read.
type LogOptions struct {
	Direction string
	Limit     int
	Cursor    string
}

func (o LogOptions) query() string {
	q := url.Values{}
	if o.Direction != "" {
		q.Set("direction", o.Direction)
	}
	// Sent whenever set, including a nonsense value: a test that asks for limit=-1
	// wants the API to reject it, not the client to quietly drop it.
	if o.Limit != 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Cursor != "" {
		q.Set("cursor", o.Cursor)
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// JobLogs reads a chunk of a build's log through core, which fetches it from the
// worker that ran the build.
func (c *Client) JobLogs(jobID string, opts LogOptions) (LogPage, error) {
	return call[LogPage](c, http.MethodGet, "/api/v1/jobs/"+jobID+"/logs"+opts.query(), nil)
}

// Executions lists finished attempts.
func (c *Client) Executions() ([]Execution, error) {
	return callField[[]Execution](c, http.MethodGet, "/api/v1/executions", "executions", nil)
}

// ListQueues returns the known queues and their depth.
func (c *Client) ListQueues() ([]QueueInfo, error) {
	return callField[[]QueueInfo](c, http.MethodGet, "/api/v1/queues", "queues", nil)
}

// QueueJobs lists the jobs waiting on a queue.
func (c *Client) QueueJobs(name string) ([]Job, error) {
	return callField[[]Job](c, http.MethodGet, "/api/v1/queues/"+name+"/jobs", "jobs", nil)
}

// PurgeQueue drops everything waiting on a queue.
func (c *Client) PurgeQueue(name string) (int64, error) {
	return callField[int64](c, http.MethodPost, "/api/v1/queues/"+name+"/purge", "deleted", nil)
}

// DLQ lists the jobs a queue gave up on.
func (c *Client) DLQ(queue string) ([]Job, error) {
	return callField[[]Job](c, http.MethodGet, "/api/v1/dlq/"+queue, "jobs", nil)
}

// RetryDLQ re-enqueues everything in a queue's dead-letter list.
func (c *Client) RetryDLQ(queue string) (int, error) {
	return callField[int](c, http.MethodPost, "/api/v1/dlq/"+queue+"/retry-all", "count", nil)
}

// ── settings and keys ─────────────────────────────────────────────────────────

// UpdateSettings writes the provided switches and returns the resulting state.
func (c *Client) UpdateSettings(update SettingsUpdate) (Settings, error) {
	return call[Settings](c, http.MethodPut, "/api/v1/settings", update)
}

// CreateAPIKey mints a key; the token is returned once.
func (c *Client) CreateAPIKey(name string) (APIKey, error) {
	return call[APIKey](c, http.MethodPost, "/api/v1/api-keys", map[string]string{"name": name})
}

// RevokeAPIKey disables a key.
func (c *Client) RevokeAPIKey(id string) error {
	_, _, err := c.request(http.MethodDelete, "/api/v1/api-keys/"+id, nil)
	return err
}

// ── public API (API key) ──────────────────────────────────────────────────────

// CreateRun starts work through the public API.
func (c *Client) CreateRun(req CreateRunRequest) (Run, error) {
	return call[Run](c, http.MethodPost, "/api/public/v1/runs", req)
}

// GetRun returns one run.
func (c *Client) GetRun(id string) (Run, error) {
	return call[Run](c, http.MethodGet, "/api/public/v1/runs/"+id, nil)
}

// ListRuns returns the caller's runs.
func (c *Client) ListRuns() ([]Run, error) {
	return callField[[]Run](c, http.MethodGet, "/api/public/v1/runs", "runs", nil)
}

// RunLogs reads a chunk of a run's log.
func (c *Client) RunLogs(id string, opts LogOptions) (LogPage, error) {
	return call[LogPage](c, http.MethodGet, "/api/public/v1/runs/"+id+"/logs"+opts.query(), nil)
}
