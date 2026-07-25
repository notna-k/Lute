package jobdefs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/queue"
)

// Handler serves Git-managed job definitions and triggers builds from them.
type Handler struct {
	defs       *repos.JobDefinitionRepository
	runs       *repos.RunRepository
	executions *repos.JobExecutionRepository
	engine     *queue.Engine
	stats      *queue.StatsAggregator
	grpcSrv    *grpc.Server
}

func NewHandler(
	defs *repos.JobDefinitionRepository,
	runs *repos.RunRepository,
	executions *repos.JobExecutionRepository,
	engine *queue.Engine,
	stats *queue.StatsAggregator,
	grpcSrv *grpc.Server,
) *Handler {
	return &Handler{defs: defs, runs: runs, executions: executions, engine: engine, stats: stats, grpcSrv: grpcSrv}
}

// --- DTOs (JSON keys match ui/src/types/jobs.ts) ---

type sourceDTO struct {
	Repo   string `json:"repo"`
	Path   string `json:"path"`
	Commit string `json:"commit"`
	InSync bool   `json:"inSync"`
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
	SuccessRate      float64                 `json:"successRate"`
	MedianDurationMs int64                   `json:"medianDurationMs"`
}

type buildDTO struct {
	// ID is the short, human-facing build reference (#a1b2c3d4).
	ID string `json:"id"`
	// RunID is the full run identifier — use this to address the build in APIs.
	RunID       string `json:"runId"`
	JobSlug     string `json:"jobSlug"`
	Status      string `json:"status"`
	Environment string `json:"environment,omitempty"`
	StartedAt   int64  `json:"startedAt"`
	DurationMs  int64  `json:"durationMs,omitempty"`
}

// containerSpec is the JSON envelope for a "container" job (matches the proto
// ContainerJobSpec field names the worker decodes).
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
			InSync: true,
		},
		Parameters:       params,
		SuccessRate:      rate,
		MedianDurationMs: median,
	}
}

// List returns all definitions with per-user build stats.
func (h *Handler) List(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	defs, err := h.defs.List(ctx)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	slugs := make([]string, 0, len(defs))
	for i := range defs {
		slugs = append(slugs, defs[i].Slug)
	}
	runsBySlug, err := h.runs.ListByJobSlugs(ctx, userID, slugs, 100)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	execs, err := h.execsFor(ctx, runsBySlug)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]jobDTO, 0, len(defs))
	for i := range defs {
		rate, median := statsOf(runsBySlug[defs[i].Slug], execs)
		out = append(out, h.toJobDTO(&defs[i], rate, median))
	}
	c.JSON(http.StatusOK, gin.H{"jobs": out})
}

// Get returns one definition.
func (h *Handler) Get(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	def, err := h.defs.GetBySlug(ctx, c.Param("slug"))
	if err != nil {
		notFoundOrInternal(c, err)
		return
	}
	runs, err := h.runs.ListByJobSlug(ctx, userID, def.Slug, 100)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	execs, err := h.executions.ListByJobIDs(ctx, jobIDsOf(runs))
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	rate, median := statsOf(runs, execs)
	c.JSON(http.StatusOK, h.toJobDTO(def, rate, median))
}

// Builds returns the recent builds (runs) for a job.
func (h *Handler) Builds(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	slug := c.Param("slug")
	runs, err := h.runs.ListByJobSlug(ctx, userID, slug, 20)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	execs, err := h.executions.ListByJobIDs(ctx, jobIDsOf(runs))
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]buildDTO, 0, len(runs))
	for i := range runs {
		out = append(out, h.buildDTO(ctx, &runs[i], execs[runs[i].JobID]))
	}
	c.JSON(http.StatusOK, gin.H{"builds": out})
}

type triggerRequest struct {
	Values map[string]any `json:"values"`
}

// Trigger validates the payload against the schema and enqueues a build.
func (h *Handler) Trigger(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	def, err := h.defs.GetBySlug(ctx, c.Param("slug"))
	if err != nil {
		notFoundOrInternal(c, err)
		return
	}

	var req triggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	resolved, verr := Validate(def.Parameters, req.Values)
	if verr != nil {
		var ve *ValidationError
		if errors.As(verr, &ve) {
			// `error` stays a plain string like every other handler in the API
			// (the UI renders it directly); `fields` carries the per-input detail.
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error":  ve.Error(),
				"code":   "invalid_parameters",
				"fields": ve.Fields,
			})
			return
		}
		writeError(c, http.StatusBadRequest, verr.Error())
		return
	}

	payload, err := json.Marshal(containerSpec{
		SourceRepository: def.SourceRepo,
		Runtime:          def.Runtime,
		RequestParams:    resolved.Env,
		Command:          def.Command,
	})
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}

	run := &models.Run{
		JobID:       uuid.New().String(),
		UserID:      userID,
		Queue:       def.Queue,
		Type:        "container",
		JobSlug:     def.Slug,
		Environment: resolved.Environment,
		Params:      resolved.Env,
	}
	if err := h.runs.Create(ctx, run); err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}

	job := &queue.Job{
		ID:       run.JobID,
		Queue:    def.Queue,
		Type:     "container",
		Payload:  payload,
		Meta:     map[string]string{"user_id": userID.Hex(), "run_id": run.ID.Hex(), "job_slug": def.Slug},
		Selector: def.LabelSelector,
	}
	if err := h.engine.Enqueue(ctx, job, queue.EnqueueOpts{}); err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.stats.RecordEnqueued(ctx, def.Queue)
	if h.grpcSrv != nil {
		h.grpcSrv.DispatchQueue(ctx, def.Queue)
	}

	c.JSON(http.StatusCreated, h.buildDTO(ctx, run, nil))
}

// buildDTO derives a build's live status from queue + execution state. exec is
// the already-loaded execution record for this run, or nil if it hasn't
// finished (see execsFor / ListByJobIDs — never query per build here).
func (h *Handler) buildDTO(ctx context.Context, run *models.Run, exec *models.JobExecution) buildDTO {
	b := buildDTO{
		ID:          shortID(run.ID),
		RunID:       run.ID.Hex(),
		JobSlug:     run.JobSlug,
		Status:      "queued",
		Environment: run.Environment,
		StartedAt:   run.CreatedAt.UTC().UnixMilli(),
	}
	if job, err := h.engine.GetJob(ctx, run.JobID); err == nil {
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

// jobIDsOf collects the queue-job IDs of a run set, for a batched execution load.
func jobIDsOf(runs []models.Run) []string {
	ids := make([]string, 0, len(runs))
	for i := range runs {
		ids = append(ids, runs[i].JobID)
	}
	return ids
}

// execsFor loads the executions for every run in a slug→runs map in one query.
func (h *Handler) execsFor(ctx context.Context, runsBySlug map[string][]models.Run) (map[string]*models.JobExecution, error) {
	var ids []string
	for _, runs := range runsBySlug {
		ids = append(ids, jobIDsOf(runs)...)
	}
	return h.executions.ListByJobIDs(ctx, ids)
}

// statsOf computes success rate and median duration over a job's builds using
// pre-loaded executions.
func statsOf(runs []models.Run, execs map[string]*models.JobExecution) (float64, int64) {
	var durations []int64
	finished, passed := 0, 0
	for i := range runs {
		exec := execs[runs[i].JobID]
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

func requireUserID(c *gin.Context) (id.ID, bool) {
	raw, exists := c.Get("user_id")
	if !exists {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return id.ID(""), false
	}
	uid, err := id.FromHex(raw.(string))
	if err != nil {
		writeError(c, http.StatusUnauthorized, "invalid user context")
		return id.ID(""), false
	}
	return uid, true
}

func notFoundOrInternal(c *gin.Context, err error) {
	if errors.Is(err, repos.ErrNotFound) {
		writeError(c, http.StatusNotFound, "job not found")
		return
	}
	writeError(c, http.StatusInternalServerError, err.Error())
}

// writeError matches the `{"error": "message"}` shape the rest of the API (and
// the UI's api client) uses — a nested object here surfaces as "[object Object]".
func writeError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}
