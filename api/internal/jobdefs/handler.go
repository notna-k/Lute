package jobdefs

import (
	"context"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"

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
