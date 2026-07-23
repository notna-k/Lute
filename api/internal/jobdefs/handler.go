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
	ID          string `json:"id"`
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
	out := make([]jobDTO, 0, len(defs))
	for i := range defs {
		rate, median := h.statsFor(ctx, userID, defs[i].Slug)
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
	rate, median := h.statsFor(ctx, userID, def.Slug)
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
	out := make([]buildDTO, 0, len(runs))
	for i := range runs {
		out = append(out, h.buildDTO(ctx, &runs[i]))
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
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_parameters", "fields": ve.Fields}})
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

	c.JSON(http.StatusCreated, h.buildDTO(ctx, run))
}

// buildDTO derives a build's live status from queue + execution state.
func (h *Handler) buildDTO(ctx context.Context, run *models.Run) buildDTO {
	b := buildDTO{
		ID:          shortID(run.ID),
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
	if h.executions != nil {
		if exec, err := h.executions.GetByJobID(ctx, run.JobID); err == nil {
			b.DurationMs = exec.ElapsedMs
			if exec.Success {
				b.Status = "passed"
			} else {
				b.Status = "failed"
			}
		}
	}
	return b
}

// statsFor computes success rate and median duration over a job's builds.
func (h *Handler) statsFor(ctx context.Context, userID id.ID, slug string) (float64, int64) {
	runs, err := h.runs.ListByJobSlug(ctx, userID, slug, 100)
	if err != nil || len(runs) == 0 {
		return 0, 0
	}
	var durations []int64
	finished, passed := 0, 0
	for i := range runs {
		exec, err := h.executions.GetByJobID(ctx, runs[i].JobID)
		if err != nil {
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

func writeError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"message": message}})
}
