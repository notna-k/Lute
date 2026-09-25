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

// Client talks to core only over HTTP, as the panel and public-API consumers do.
type Client struct {
	t       *testing.T
	BaseURL string
	http    *http.Client

	accessToken string
	apiKey      string
}

// NewClient has its own cookie jar, so the refresh cookie behaves as in a browser.
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

func (c *Client) WithAPIKey(token string) *Client {
	clone := *c
	clone.apiKey = token
	clone.accessToken = ""
	return &clone
}

func (c *Client) Token() string { return c.accessToken }

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

func StatusOf(err error) int {
	var apiErr *APIError
	if err != nil && errors.As(err, &apiErr) {
		return apiErr.Status
	}
	return 0
}

func CodeOf(err error) string {
	var apiErr *APIError
	if err != nil && errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return ""
}

func FieldsOf(err error) map[string]string {
	var apiErr *APIError
	if err != nil && errors.As(err, &apiErr) {
		return apiErr.Fields
	}
	return nil
}

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
	var body struct {
		Error struct {
			Code    string            `json:"code"`
			Message string            `json:"message"`
			Fields  map[string]string `json:"fields"`
		} `json:"error"`
	}
	apiErr := &APIError{Status: status, Body: string(raw)}
	if err := json.Unmarshal(raw, &body); err == nil {
		apiErr.Code = body.Error.Code
		apiErr.Message = body.Error.Message
		apiErr.Fields = body.Error.Fields
	}
	if apiErr.Message == "" {
		apiErr.Message = strings.TrimSpace(string(raw))
	}
	return apiErr
}

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

func (c *Client) Login(email, password string) (Session, error) {
	s, err := call[Session](c, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": email, "password": password})
	if err == nil {
		c.accessToken = s.AccessToken
	}
	return s, err
}

func (c *Client) Refresh() (Session, error) {
	s, err := call[Session](c, http.MethodPost, "/api/v1/auth/refresh", nil)
	if err == nil {
		c.accessToken = s.AccessToken
	}
	return s, err
}

func (c *Client) Logout() error {
	_, _, err := c.request(http.MethodPost, "/api/v1/auth/logout", nil)
	return err
}

func (c *Client) Me() (User, error) {
	return call[User](c, http.MethodGet, "/api/v1/auth/me", nil)
}

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

func (c *Client) SetRefreshCookie(value string) {
	u, err := url.Parse(c.BaseURL + "/api/v1/auth")
	if err != nil {
		return
	}
	c.http.Jar.SetCookies(u, []*http.Cookie{{Name: "lute_refresh", Value: value, Path: "/api/v1/auth"}})
}

func (c *Client) CreateClaimCode() (ClaimCode, error) {
	return call[ClaimCode](c, http.MethodPost, "/api/v1/workers/claim-code", nil)
}

// RegisterWorker is the unauthenticated call an agent makes with its claim code.
func (c *Client) RegisterWorker(req WorkerRegistration) (Registered, error) {
	return call[Registered](c, http.MethodPost, "/api/public/v1/workers/bootstrap/register", req)
}

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

func (c *Client) GetWorker(id string) (Worker, error) {
	return call[Worker](c, http.MethodGet, "/api/v1/workers/"+id, nil)
}

func (c *Client) WorkerStatus(id string) (WorkerLiveStatus, error) {
	return call[WorkerLiveStatus](c, http.MethodGet, "/api/v1/workers/"+id+"/status", nil)
}

func (c *Client) ConnectedWorkers() ([]ConnectedWorker, error) {
	return callField[[]ConnectedWorker](c, http.MethodGet, "/api/v1/workers/connected", "workers", nil)
}

func (c *Client) PatchLabels(id string, labels map[string]string) (Worker, error) {
	return call[Worker](c, http.MethodPatch, "/api/v1/workers/"+id+"/labels",
		map[string]any{"labels": labels})
}

func (c *Client) DeleteWorker(id string) error {
	_, _, err := c.request(http.MethodDelete, "/api/v1/workers/"+id, nil)
	return err
}

func (c *Client) ReEnableWorker(id string) (Worker, error) {
	return call[Worker](c, http.MethodPost, "/api/v1/workers/"+id+"/re-enable", nil)
}

func (c *Client) ListJobDefs() ([]JobDefinition, error) {
	return callField[[]JobDefinition](c, http.MethodGet, "/api/v1/job-definitions", "jobs", nil)
}

func (c *Client) GetJobDef(slug string) (JobDefinition, error) {
	return call[JobDefinition](c, http.MethodGet, "/api/v1/job-definitions/"+slug, nil)
}

func (c *Client) CreateJobDef(req JobDefinitionRequest) (JobDefinition, error) {
	return call[JobDefinition](c, http.MethodPost, "/api/v1/job-definitions", req)
}

func (c *Client) UpdateJobDef(slug string, req JobDefinitionRequest) (JobDefinition, error) {
	return call[JobDefinition](c, http.MethodPut, "/api/v1/job-definitions/"+slug, req)
}

func (c *Client) SyncJobDefs() (SyncResult, error) {
	return call[SyncResult](c, http.MethodPost, "/api/v1/job-definitions/sync", nil)
}

func (c *Client) RevertJobDef(slug string) (JobDefinition, error) {
	return call[JobDefinition](c, http.MethodPost, "/api/v1/job-definitions/"+slug+"/revert", nil)
}

func (c *Client) ExportJobDef(slug string) (string, error) {
	_, raw, err := c.request(http.MethodGet, "/api/v1/job-definitions/"+slug+"/yaml", nil)
	return string(raw), err
}

func (c *Client) ExportJobDefsZip() ([]byte, error) {
	_, raw, err := c.request(http.MethodGet, "/api/v1/job-definitions/export.zip", nil)
	return raw, err
}

func (c *Client) Trigger(slug string, values map[string]any) (Build, error) {
	return call[Build](c, http.MethodPost, "/api/v1/job-definitions/"+slug+"/trigger",
		TriggerRequest{Values: values})
}

// TriggerWithSchema sends its own schema, which makes the build ad-hoc if it differs from Git.
func (c *Client) TriggerWithSchema(slug string, values map[string]any, params []ParameterField) (Build, error) {
	return call[Build](c, http.MethodPost, "/api/v1/job-definitions/"+slug+"/trigger",
		TriggerRequest{Values: values, Parameters: params})
}

func (c *Client) Builds(slug string) ([]Build, error) {
	return callField[[]Build](c, http.MethodGet, "/api/v1/job-definitions/"+slug+"/builds", "builds", nil)
}

func (c *Client) Enqueue(req EnqueueRequest) (Enqueued, error) {
	return call[Enqueued](c, http.MethodPost, "/api/v1/jobs", req)
}

func (c *Client) GetJob(id string) (Job, error) {
	return call[Job](c, http.MethodGet, "/api/v1/jobs/"+id, nil)
}

func (c *Client) CancelJob(id string) error {
	_, _, err := c.request(http.MethodDelete, "/api/v1/jobs/"+id, nil)
	return err
}

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
	// Sent even when nonsensical: a test asking for limit=-1 wants the API to reject it.
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

func (c *Client) JobLogs(jobID string, opts LogOptions) (LogPage, error) {
	return call[LogPage](c, http.MethodGet, "/api/v1/jobs/"+jobID+"/logs"+opts.query(), nil)
}

func (c *Client) Executions() ([]Execution, error) {
	return callField[[]Execution](c, http.MethodGet, "/api/v1/executions", "executions", nil)
}

func (c *Client) ListQueues() ([]QueueInfo, error) {
	return callField[[]QueueInfo](c, http.MethodGet, "/api/v1/queues", "queues", nil)
}

func (c *Client) QueueJobs(name string) ([]Job, error) {
	return callField[[]Job](c, http.MethodGet, "/api/v1/queues/"+name+"/jobs", "jobs", nil)
}

func (c *Client) PurgeQueue(name string) (int64, error) {
	return callField[int64](c, http.MethodPost, "/api/v1/queues/"+name+"/purge", "deleted", nil)
}

func (c *Client) DLQ(queue string) ([]Job, error) {
	return callField[[]Job](c, http.MethodGet, "/api/v1/dlq/"+queue, "jobs", nil)
}

func (c *Client) RetryDLQ(queue string) (int, error) {
	return callField[int](c, http.MethodPost, "/api/v1/dlq/"+queue+"/retry-all", "count", nil)
}

func (c *Client) UpdateSettings(update SettingsUpdate) (Settings, error) {
	return call[Settings](c, http.MethodPut, "/api/v1/settings", update)
}

func (c *Client) CreateAPIKey(name string) (APIKey, error) {
	return call[APIKey](c, http.MethodPost, "/api/v1/api-keys", map[string]string{"name": name})
}

func (c *Client) RevokeAPIKey(id string) error {
	_, _, err := c.request(http.MethodDelete, "/api/v1/api-keys/"+id, nil)
	return err
}

func (c *Client) CreateRun(req CreateRunRequest) (Run, error) {
	return call[Run](c, http.MethodPost, "/api/public/v1/runs", req)
}

func (c *Client) GetRun(id string) (Run, error) {
	return call[Run](c, http.MethodGet, "/api/public/v1/runs/"+id, nil)
}

func (c *Client) ListRuns() ([]Run, error) {
	return callField[[]Run](c, http.MethodGet, "/api/public/v1/runs", "runs", nil)
}

func (c *Client) RunLogs(id string, opts LogOptions) (LogPage, error) {
	return call[LogPage](c, http.MethodGet, "/api/public/v1/runs/"+id+"/logs"+opts.query(), nil)
}
