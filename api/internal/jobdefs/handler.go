package jobdefs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/httpx"
	"github.com/lute/api/internal/runs"
)

type Handler struct {
	defs     *repos.JobDefinitionRepository
	syncer   *Syncer
	settings *repos.SettingRepository
	runs     *runs.Service
}

func NewHandler(defs *repos.JobDefinitionRepository, syncer *Syncer, settings *repos.SettingRepository, svc *runs.Service) *Handler {
	return &Handler{defs: defs, syncer: syncer, settings: settings, runs: svc}
}

// JSON keys below match ui/src/types/jobs.ts.

type sourceDTO struct {
	Repo   string `json:"repo"`
	Path   string `json:"path"`
	Commit string `json:"commit"`
}

type jobDTO struct {
	Slug             string                  `json:"slug"`
	Name             string                  `json:"name"`
	Description      string                  `json:"description"`
	Queue            string                  `json:"queue"`
	LabelSelector    map[string]string       `json:"labelSelector"`
	Runtime          string                  `json:"runtime"`
	Command          string                  `json:"command"`
	Source           sourceDTO               `json:"source"`
	Parameters       []models.ParameterField `json:"parameters"`
	GitState         string                  `json:"gitState"`
	SuccessRate      float64                 `json:"successRate"`
	MedianDurationMs int64                   `json:"medianDurationMs"`
	// LastBuild saves the job list a request per row.
	LastBuild *buildDTO `json:"lastBuild,omitempty"`
	// Recent is the trailing build statuses, oldest first.
	Recent []string `json:"recent,omitempty"`
}

const recentWindow = 16

type buildDTO struct {
	// ID is the short, human-facing reference (#a1b2c3d4).
	ID    string `json:"id"`
	RunID string `json:"runId"`
	// JobID addresses the build in the queue and log APIs; it is not interchangeable with RunID.
	JobID       string `json:"jobId"`
	JobSlug     string `json:"jobSlug"`
	Status      string `json:"status"`
	Environment string `json:"environment,omitempty"`
	StartedAt   int64  `json:"startedAt"`
	DurationMs  int64  `json:"durationMs,omitempty"`
	// Params are keyed by env var. Secret parameters never reach Run.Params, so none are echoed.
	Params map[string]string `json:"params,omitempty"`
	// AdHoc marks a build that ran a panel-edited schema instead of the one in Git.
	AdHoc bool `json:"adHoc,omitempty"`
}

// containerSpec's field names match the proto ContainerJobSpec the worker decodes.
type containerSpec struct {
	SourceRepository string            `json:"source_repository,omitempty"`
	Runtime          string            `json:"runtime"`
	RequestParams    map[string]string `json:"request_params,omitempty"`
	Command          string            `json:"command"`
}

func (h *Handler) toJobDTO(def *models.JobDefinition, rate float64, median int64) jobDTO {
	labels := def.LabelSelector
	if labels == nil {
		labels = map[string]string{}
	}
	params := def.Parameters
	if params == nil {
		params = []models.ParameterField{}
	}
	return jobDTO{
		Slug:          def.Slug,
		Name:          def.Name,
		Description:   def.Description,
		Queue:         def.Queue,
		LabelSelector: labels,
		Runtime:       def.Runtime,
		Command:       def.Command,
		Source: sourceDTO{
			Repo:   def.SourceRepo,
			Path:   def.SourcePath,
			Commit: def.SourceCommit,
		},
		GitState:         def.GitState(),
		Parameters:       params,
		SuccessRate:      rate,
		MedianDurationMs: median,
	}
}

// withHistory attaches the newest build and the status strip. history must be newest first.
func (h *Handler) withHistory(ctx context.Context, dto jobDTO, history []models.Run, execs map[string]*models.JobExecution) jobDTO {
	if len(history) == 0 {
		return dto
	}
	last := h.buildDTO(ctx, &history[0], execs[history[0].JobID])
	dto.LastBuild = &last
	dto.Recent = recentStatuses(history, execs, last.Status)
	return dto
}

// recentStatuses returns trailing build statuses, oldest first. Only the newest is
// resolved against the queue (lastStatus); for the rest, no execution record means
// the build never finished.
func recentStatuses(history []models.Run, execs map[string]*models.JobExecution, lastStatus string) []string {
	window := history
	if len(window) > recentWindow {
		window = window[:recentWindow]
	}
	out := make([]string, 0, len(window))
	for i := len(window) - 1; i >= 0; i-- {
		switch {
		case i == 0:
			out = append(out, lastStatus)
		case execs[window[i].JobID] == nil:
			out = append(out, "queued")
		case execs[window[i].JobID].Success:
			out = append(out, "passed")
		default:
			out = append(out, "failed")
		}
	}
	return out
}

func (h *Handler) List(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	defs, err := h.defs.List(ctx)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	slugs := make([]string, 0, len(defs))
	for i := range defs {
		slugs = append(slugs, defs[i].Slug)
	}
	runsBySlug, execs, err := h.runs.History(ctx, userID, slugs, 100)
	if err != nil {
		httpx.Internal(c, err)
		return
	}

	out := make([]jobDTO, 0, len(defs))
	for i := range defs {
		history := runsBySlug[defs[i].Slug]
		rate, median := statsOf(history, execs)
		out = append(out, h.withHistory(ctx, h.toJobDTO(&defs[i], rate, median), history, execs))
	}
	c.JSON(http.StatusOK, gin.H{"jobs": out})
}

func (h *Handler) Get(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	def, err := h.defs.GetBySlug(ctx, c.Param("slug"))
	if err != nil {
		httpx.NotFoundOrInternal(c, err, "job not found")
		return
	}
	bySlug, execs, err := h.runs.History(ctx, userID, []string{def.Slug}, 100)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	history := bySlug[def.Slug]
	rate, median := statsOf(history, execs)
	c.JSON(http.StatusOK, h.withHistory(ctx, h.toJobDTO(def, rate, median), history, execs))
}

func (h *Handler) Builds(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	slug := c.Param("slug")
	bySlug, execs, err := h.runs.History(ctx, userID, []string{slug}, 20)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	history := bySlug[slug]
	out := make([]buildDTO, 0, len(history))
	for i := range history {
		out = append(out, h.buildDTO(ctx, &history[i], execs[history[i].JobID]))
	}
	c.JSON(http.StatusOK, gin.H{"builds": out})
}

type triggerRequest struct {
	Values map[string]any `json:"values"`
	// Parameters is the schema the panel rendered. If it differs from Git this is an
	// ad-hoc build validated against it; omit it to run the definition as committed.
	Parameters []models.ParameterField `json:"parameters"`
}

func (h *Handler) Trigger(c *gin.Context) {
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	def, err := h.defs.GetBySlug(ctx, c.Param("slug"))
	if err != nil {
		httpx.NotFoundOrInternal(c, err, "job not found")
		return
	}

	var req triggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	schema := def.Parameters
	adhoc := false
	if req.Parameters != nil && schemaDiffers(def.Parameters, req.Parameters) {
		allowed, err := h.settings.GetBool(ctx, models.AllowAdhocBuilds)
		if err != nil {
			httpx.Internal(c, err)
			return
		}
		if !allowed {
			httpx.Error(c, http.StatusConflict, "this build's parameters differ from the definition in Git, "+
				"and ad-hoc builds are turned off — commit your changes, or enable ad-hoc builds in Settings")
			return
		}
		schema = req.Parameters
		adhoc = true
	}

	resolved, verr := Validate(schema, req.Values)
	if verr != nil {
		var ve *ValidationError
		if errors.As(verr, &ve) {
			httpx.Invalid(c, ve.Error(), ve.Fields)
			return
		}
		httpx.Error(c, http.StatusBadRequest, verr.Error())
		return
	}

	payload, err := json.Marshal(containerSpec{
		SourceRepository: def.SourceRepo,
		Runtime:          def.Runtime,
		RequestParams:    resolved.Env,
		Command:          def.Command,
	})
	if err != nil {
		httpx.Internal(c, err)
		return
	}

	run := &models.Run{
		UserID:      userID,
		Queue:       def.Queue,
		Type:        "container",
		JobSlug:     def.Slug,
		Environment: resolved.Environment,
		Params:      resolved.Env,
		AdHoc:       adhoc,
	}
	if adhoc {
		run.ParamSchema = schema
	}
	run, _, err = h.runs.Enqueue(ctx, run, runs.Job{Payload: payload, Selector: def.LabelSelector})
	if err != nil {
		httpx.Internal(c, err)
		return
	}

	c.JSON(http.StatusCreated, h.buildDTO(ctx, run, nil))
}

// createRequest is a template authored in the panel.
type createRequest struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Queue       string                  `json:"queue"`
	Runtime     string                  `json:"runtime"`
	Command     string                  `json:"command"`
	SourceRepo  string                  `json:"sourceRepo"`
	Labels      map[string]string       `json:"labelSelector"`
	Parameters  []models.ParameterField `json:"parameters"`
}

// validate returns a message and what is wrong per input, or nil fields if the template can be saved.
func (r createRequest) validate() (string, map[string]string) {
	var msgs []string
	fields := map[string]string{}
	add := func(field, msg string) {
		if _, seen := fields[field]; !seen {
			fields[field] = msg
			msgs = append(msgs, msg)
		}
	}
	for _, f := range []struct{ name, value string }{{"name", r.Name}, {"runtime", r.Runtime}, {"command", r.Command}} {
		if strings.TrimSpace(f.value) == "" {
			add(f.name, f.name+" is required")
		}
	}
	for _, p := range r.Parameters {
		if strings.TrimSpace(p.Name) == "" {
			add("parameters", "every parameter needs a name")
		} else if !KnownTypes[p.Type] {
			add("parameters", fmt.Sprintf("parameter %q has unknown type %q", p.Name, p.Type))
		}
	}
	if len(msgs) == 0 {
		return "", nil
	}
	return strings.Join(msgs, "; "), fields
}

func (r createRequest) spec() models.JobSpec {
	queueName := strings.TrimSpace(r.Queue)
	if queueName == "" {
		queueName = "default"
	}
	return models.JobSpec{
		Name:          r.Name,
		Description:   r.Description,
		Queue:         queueName,
		LabelSelector: r.Labels,
		Runtime:       r.Runtime,
		Command:       r.Command,
		SourceRepo:    r.SourceRepo,
		Parameters:    r.Parameters,
	}
}

// Create saves a panel-authored template. With no Git snapshot it shows as "not in
// Git" until a file with its slug is committed, or a pruning sync deletes it.
func (h *Handler) Create(c *gin.Context) {
	if _, ok := httpx.UserID(c); !ok {
		return
	}
	ctx := c.Request.Context()

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if msg, fields := req.validate(); fields != nil {
		httpx.Invalid(c, msg, fields)
		return
	}

	slug := slugify(req.Name)
	if slug == "" {
		httpx.Error(c, http.StatusBadRequest, "could not derive a slug from the name")
		return
	}
	if _, err := h.defs.GetBySlug(ctx, slug); err == nil {
		httpx.Error(c, http.StatusConflict, fmt.Sprintf("a job definition named %q already exists", slug))
		return
	} else if !errors.Is(err, repos.ErrNotFound) {
		httpx.Internal(c, err)
		return
	}

	def := &models.JobDefinition{Slug: slug, JobSpec: req.spec()}
	if err := h.defs.Create(ctx, def); err != nil {
		httpx.Internal(c, err)
		return
	}
	c.JSON(http.StatusCreated, h.toJobDTO(def, 0, 0))
}

// Update rewrites a definition's spec. A Git definition drifts until its file changes.
func (h *Handler) Update(c *gin.Context) {
	if _, ok := httpx.UserID(c); !ok {
		return
	}
	ctx := c.Request.Context()

	def, err := h.defs.GetBySlug(ctx, c.Param("slug"))
	if err != nil {
		httpx.NotFoundOrInternal(c, err, "job not found")
		return
	}
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if msg, fields := req.validate(); fields != nil {
		httpx.Invalid(c, msg, fields)
		return
	}

	// The slug stays: runs reference it, and a rename must not orphan build history.
	def.JobSpec = req.spec()

	if err := h.defs.Update(ctx, def); err != nil {
		httpx.NotFoundOrInternal(c, err, "job not found")
		return
	}
	c.JSON(http.StatusOK, h.toJobDTO(def, 0, 0))
}

// buildDTO derives a build's live status from queue and execution state. exec is
// preloaded (nil if unfinished); never query per build here.
func (h *Handler) buildDTO(ctx context.Context, run *models.Run, exec *models.JobExecution) buildDTO {
	b := buildDTO{
		ID:          shortID(run.ID),
		RunID:       run.ID.Hex(),
		JobID:       run.JobID,
		JobSlug:     run.JobSlug,
		Status:      "queued",
		Environment: run.Environment,
		StartedAt:   run.CreatedAt.UTC().UnixMilli(),
		Params:      run.Params,
		AdHoc:       run.AdHoc,
	}
	if job, err := h.runs.Job(ctx, run.JobID); err == nil {
		switch job.Status {
		case "running":
			b.Status = "running"
		case "done":
			b.Status = "passed"
		case "dead":
			b.Status = "failed"
		default:
			b.Status = "queued"
		}
		if job.StartedAt > 0 {
			b.StartedAt = job.StartedAt * 1000
		}
		// Covers the window before the execution record is written.
		if job.ElapsedMs > 0 {
			b.DurationMs = job.ElapsedMs
		}
	}
	if exec != nil {
		b.DurationMs = exec.ElapsedMs
		if exec.Success {
			b.Status = "passed"
		} else {
			b.Status = "failed"
		}
	}
	return b
}

// statsOf returns success rate and median duration over a job's builds.
func statsOf(history []models.Run, execs map[string]*models.JobExecution) (float64, int64) {
	var durations []int64
	finished, passed := 0, 0
	for i := range history {
		exec := execs[history[i].JobID]
		if exec == nil {
			continue
		}
		finished++
		if exec.Success {
			passed++
		}
		if exec.ElapsedMs > 0 {
			durations = append(durations, exec.ElapsedMs)
		}
	}
	if finished == 0 {
		return 0, 0
	}
	rate := float64(passed) / float64(finished)
	var median int64
	if len(durations) > 0 {
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		median = durations[len(durations)/2]
	}
	return rate, median
}

func shortID(i id.ID) string {
	s := i.Hex()
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
