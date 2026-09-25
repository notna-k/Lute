package jobdefs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/httpx"
	"github.com/lute/api/internal/runs"
)

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

func shortID(i id.ID) string {
	s := i.Hex()
	if len(s) > 8 {
		return s[:8]
	}
	return s
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
