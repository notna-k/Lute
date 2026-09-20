package jobdefs

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/models"
)

// Sync reconciles definitions with Git on demand, rather than waiting for the
// next restart.
func (h *Handler) Sync(c *gin.Context) {
	res, err := h.syncer.Sync(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, res)
}

// Export returns every definition as one multi-document YAML stream — the
// panel's current state, ready to commit so it becomes canonical.
func (h *Handler) Export(c *gin.Context) {
	defs, err := h.defs.List(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	writeYAML(c, defs)
}

// ExportOne returns one definition's YAML.
func (h *Handler) ExportOne(c *gin.Context) {
	def, err := h.defs.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		notFoundOrInternal(c, err)
		return
	}
	writeYAML(c, []models.JobDefinition{*def})
}

// Revert discards panel edits, restoring the spec Git last stated.
func (h *Handler) Revert(c *gin.Context) {
	ctx := c.Request.Context()
	def, err := h.defs.GetBySlug(ctx, c.Param("slug"))
	if err != nil {
		notFoundOrInternal(c, err)
		return
	}
	if def.GitSpec == nil {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error": "this definition is not in Git — there is nothing to revert to",
			"code":  "not_in_git",
		})
		return
	}
	def.JobSpec = *def.GitSpec
	if err := h.defs.Update(ctx, def); err != nil {
		notFoundOrInternal(c, err)
		return
	}
	c.JSON(http.StatusOK, h.toJobDTO(def, 0, 0))
}

// writeYAML wraps the export in JSON, like every other response the panel's
// API client reads.
func writeYAML(c *gin.Context, defs []models.JobDefinition) {
	out, err := Export(defs)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"yaml": string(out)})
}
