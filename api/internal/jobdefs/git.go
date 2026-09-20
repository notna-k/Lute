package jobdefs

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

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

// ExportZip returns the same definitions as a zip of one YAML file per job,
// laid out at the paths they belong at — unpack it over the job-definitions
// repo and commit.
func (h *Handler) ExportZip(c *gin.Context) {
	defs, err := h.defs.List(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	files, err := ExportSplit(defs)
	if err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	// Built in memory so a failure halfway is still a JSON error, not a
	// truncated download. Definitions are text, and there are tens of them.
	var buf bytes.Buffer
	if err := writeZip(&buf, files, time.Now()); err != nil {
		writeError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Header("Content-Disposition", `attachment; filename="jobdefs.zip"`)
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

// writeZip packs one deflated entry per file, every entry stamped with one
// timestamp so an archive reads as a single snapshot.
func writeZip(w io.Writer, files []ExportFile, at time.Time) error {
	zw := zip.NewWriter(w)
	for _, f := range files {
		entry, err := zw.CreateHeader(&zip.FileHeader{
			Name:     f.Path,
			Method:   zip.Deflate,
			Modified: at,
		})
		if err != nil {
			return fmt.Errorf("zip %s: %w", f.Path, err)
		}
		if _, err := entry.Write(f.Body); err != nil {
			return fmt.Errorf("zip %s: %w", f.Path, err)
		}
	}
	return zw.Close()
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
