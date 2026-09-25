package jobdefs

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/httpx"
)

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
