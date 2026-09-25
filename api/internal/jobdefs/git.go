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
	"github.com/lute/api/internal/httpx"
)

func (h *Handler) Sync(c *gin.Context) {
	res, err := h.syncer.Sync(c.Request.Context())
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) Export(c *gin.Context) {
	defs, err := h.defs.List(c.Request.Context())
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	writeYAML(c, defs)
}

func (h *Handler) ExportOne(c *gin.Context) {
	def, err := h.defs.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		httpx.NotFoundOrInternal(c, err, "job not found")
		return
	}
	writeYAML(c, []models.JobDefinition{*def})
}

// ExportZip returns one YAML file per job at its Git path, ready to unpack over the repo.
func (h *Handler) ExportZip(c *gin.Context) {
	defs, err := h.defs.List(c.Request.Context())
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	files, err := ExportSplit(defs)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	// Built in memory so a failure halfway is a JSON error, not a truncated download.
	var buf bytes.Buffer
	if err := writeZip(&buf, files, time.Now()); err != nil {
		httpx.Internal(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="jobdefs.zip"`)
	c.Data(http.StatusOK, "application/zip", buf.Bytes())
}

// writeZip stamps every entry with one timestamp so the archive reads as one snapshot.
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

// Revert discards panel edits, restoring the spec from Git.
func (h *Handler) Revert(c *gin.Context) {
	ctx := c.Request.Context()
	def, err := h.defs.GetBySlug(ctx, c.Param("slug"))
	if err != nil {
		httpx.NotFoundOrInternal(c, err, "job not found")
		return
	}
	if def.GitSpec == nil {
		httpx.Error(c, http.StatusConflict, "this definition is not in Git — there is nothing to revert to")
		return
	}
	def.JobSpec = *def.GitSpec
	if err := h.defs.Update(ctx, def); err != nil {
		httpx.NotFoundOrInternal(c, err, "job not found")
		return
	}
	c.JSON(http.StatusOK, h.toJobDTO(def, 0, 0))
}

// writeYAML wraps the export in JSON, like every other response the panel reads.
func writeYAML(c *gin.Context, defs []models.JobDefinition) {
	out, err := Export(defs)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"yaml": string(out)})
}
