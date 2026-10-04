package publicapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/httpx"
	"github.com/lute/api/internal/jobdefs"
	"github.com/lute/api/internal/runs"
)

// JobsHandler serves job definitions to API keys: list them, read one's parameter
// schema, and start a run of one.
type JobsHandler struct {
	defs *repos.JobDefinitionRepository
	runs *runs.Service
}

func NewJobsHandler(defs *repos.JobDefinitionRepository, svc *runs.Service) *JobsHandler {
	return &JobsHandler{defs: defs, runs: svc}
}

type JobSummary struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Queue       string `json:"queue"`
	Runtime     string `json:"runtime"`
	// LastRun is the newest run the caller can see.
	LastRun *RunResponse `json:"last_run,omitempty"`
}

type JobDetail struct {
	JobSummary
	Command    string              `json:"command"`
	SourceRepo string              `json:"source_repo,omitempty"`
	Parameters []ParameterResponse `json:"parameters"`
}

// ParameterResponse is one input of a job. A secret parameter is listed, but nothing
// about its value is: it resolves on the worker.
type ParameterResponse struct {
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	Label       string           `json:"label,omitempty"`
	EnvVar      string           `json:"env_var"`
	Description string           `json:"description,omitempty"`
	Required    bool             `json:"required"`
	Default     any              `json:"default,omitempty"`
	Options     []OptionResponse `json:"options,omitempty"`
}

type OptionResponse struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
	Hint  string `json:"hint,omitempty"`
}

type StartRunRequest struct {
	// Params are keyed by parameter name. Lists are JSON arrays; numbers and booleans
	// may also be strings.
	Params map[string]any `json:"params"`
}

func (h *JobsHandler) List(c *gin.Context) {
	key, ok := httpx.Key(c)
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
	latest, _, err := h.runs.History(ctx, viewerOf(key), slugs, 1)
	if err != nil {
		httpx.Internal(c, err)
		return
	}

	out := make([]JobSummary, 0, len(defs))
	for i := range defs {
		s := jobSummary(&defs[i])
		if history := latest[defs[i].Slug]; len(history) > 0 {
			last := runResponse(ctx, h.runs, &history[0])
			s.LastRun = &last
		}
		out = append(out, s)
	}
	c.JSON(http.StatusOK, gin.H{"jobs": out})
}

func (h *JobsHandler) Get(c *gin.Context) {
	key, ok := httpx.Key(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	def, err := h.defs.GetBySlug(ctx, c.Param("slug"))
	if err != nil {
		httpx.NotFoundOrInternal(c, err, "job not found")
		return
	}
	latest, _, err := h.runs.History(ctx, viewerOf(key), []string{def.Slug}, 1)
	if err != nil {
		httpx.Internal(c, err)
		return
	}

	d := JobDetail{
		JobSummary: jobSummary(def),
		Command:    def.Command,
		SourceRepo: def.SourceRepo,
		Parameters: parameters(def.Parameters),
	}
	if history := latest[def.Slug]; len(history) > 0 {
		last := runResponse(ctx, h.runs, &history[0])
		d.LastRun = &last
	}
	c.JSON(http.StatusOK, d)
}

// StartRun runs the definition as it is, validating params like the panel does. An
// Idempotency-Key header that was used before returns that run with 200.
func (h *JobsHandler) StartRun(c *gin.Context) {
	key, ok := httpx.Key(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	def, err := h.defs.GetBySlug(ctx, c.Param("slug"))
	if err != nil {
		httpx.NotFoundOrInternal(c, err, "job not found")
		return
	}

	var req StartRunRequest
	// An empty body means no params, which a job without required inputs accepts.
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	run := &models.Run{
		UserID:         key.UserID,
		APIKeyID:       key.KeyID,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
	}
	run, created, err := jobdefs.Start(ctx, h.runs, def, def.Parameters, req.Params, run)
	if err != nil {
		var ve *jobdefs.ValidationError
		if errors.As(err, &ve) {
			httpx.Invalid(c, ve.Error(), ve.Fields)
			return
		}
		httpx.Internal(c, err)
		return
	}

	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	c.JSON(status, runResponse(ctx, h.runs, run))
}

func jobSummary(def *models.JobDefinition) JobSummary {
	return JobSummary{
		Slug:        def.Slug,
		Name:        def.Name,
		Description: def.Description,
		Queue:       def.Queue,
		Runtime:     def.Runtime,
	}
}

// parameters is never nil, so a job without inputs answers "parameters": [].
func parameters(fields []models.ParameterField) []ParameterResponse {
	out := make([]ParameterResponse, 0, len(fields))
	for _, f := range fields {
		p := ParameterResponse{
			Name:        f.Name,
			Type:        f.Type,
			Label:       f.Label,
			EnvVar:      f.EnvName(),
			Description: f.Description,
			Required:    f.Required,
		}
		if f.Type != "secret" {
			p.Default = f.Default
		}
		for _, o := range f.Options {
			p.Options = append(p.Options, OptionResponse{Value: o.Value, Label: o.Label, Hint: o.Hint})
		}
		out = append(out, p)
	}
	return out
}
