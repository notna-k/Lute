package worker

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
)

var labelKeyRe = regexp.MustCompile(`^[a-zA-Z0-9_\-.]{1,63}$`)

// patchLabelsRequest is the body for PATCH /api/v1/workers/:id/labels.
type patchLabelsRequest struct {
	Labels map[string]string `json:"labels" binding:"required"`
}

func validateLabels(labels map[string]string) error {
	if len(labels) > 32 {
		return fmt.Errorf("too many labels: max 32, got %d", len(labels))
	}
	for k, v := range labels {
		if !labelKeyRe.MatchString(k) {
			return fmt.Errorf("invalid label key %q: must be 1-63 chars, alphanumeric, underscore, hyphen or dot", k)
		}
		if len(v) > 255 {
			return fmt.Errorf("label value for key %q exceeds 255 characters", k)
		}
	}
	return nil
}

// GetLabels returns the label set for a worker.
func (h *WorkerHandler) GetLabels(c *gin.Context) {
	wid, err := id.FromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid worker ID"})
		return
	}
	w, err := h.workerService.GetByID(c.Request.Context(), wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "worker not found"})
		return
	}
	labels := w.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	c.JSON(http.StatusOK, gin.H{"labels": labels})
}

// PatchLabels atomically replaces the full label set for a worker.
func (h *WorkerHandler) PatchLabels(c *gin.Context) {
	wid, err := id.FromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid worker ID"})
		return
	}
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userIDObj, err := id.FromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req patchLabelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateLabels(req.Labels); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	w, err := h.workerService.GetByID(ctx, wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "worker not found"})
		return
	}
	if w.UserID != userIDObj {
		c.JSON(http.StatusForbidden, gin.H{"error": "worker not found"})
		return
	}

	w.Labels = req.Labels
	updated, err := h.workerService.Update(ctx, wid, userIDObj, w)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Keep in-memory dispatch state current and re-evaluate pending selector jobs.
	if h.connectionMgr != nil {
		h.connectionMgr.UpdateWorkerLabels(wid.Hex(), req.Labels)
	}
	if h.grpcServer != nil {
		conn := h.connectionMgr.Get(wid.Hex())
		if conn != nil {
			for _, q := range conn.Queues {
				h.grpcServer.DispatchQueue(context.Background(), q)
			}
		}
	}

	c.JSON(http.StatusOK, updated)
}

func (h *WorkerHandler) CreateWorker(c *gin.Context) {
	var w models.Worker
	if err := c.ShouldBindJSON(&w); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userIDObj, err := id.FromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	created, err := h.workerService.Create(c.Request.Context(), userIDObj, &w)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *WorkerHandler) GetWorker(c *gin.Context) {
	wid, err := id.FromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid worker ID"})
		return
	}
	w, err := h.workerService.GetByID(c.Request.Context(), wid)
	if err != nil {
		if err.Error() == "worker not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, w)
}

func (h *WorkerHandler) ListUserWorkers(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userIDObj, err := id.FromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Build label filter from repeated ?label=key:value query params.
	filter := map[string]string{}
	for _, lv := range c.QueryArray("label") {
		if k, v, ok := strings.Cut(lv, ":"); ok {
			filter[k] = v
		}
	}

	list, err := h.workerRepo.GetByUserIDAndLabels(c.Request.Context(), userIDObj, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *WorkerHandler) UpdateWorker(c *gin.Context) {
	wid, err := id.FromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid worker ID"})
		return
	}
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userIDObj, err := id.FromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	var w models.Worker
	if err := c.ShouldBindJSON(&w); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := h.workerService.Update(c.Request.Context(), wid, userIDObj, &w)
	if err != nil {
		if err.Error() == "worker not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "unauthorized: worker does not belong to user" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *WorkerHandler) ReEnableWorker(c *gin.Context) {
	wid, err := id.FromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid worker ID"})
		return
	}
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userIDObj, err := id.FromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	existing, err := h.workerService.GetByID(c.Request.Context(), wid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "worker not found"})
		return
	}
	if existing.UserID != userIDObj {
		c.JSON(http.StatusForbidden, gin.H{"error": "worker not found"})
		return
	}
	if existing.Status != "dead" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "worker is not dead; only dead workers can be re-enabled"})
		return
	}
	if err := h.workerService.UpdateStatus(c.Request.Context(), wid, "pending"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	updated, _ := h.workerService.GetByID(c.Request.Context(), wid)
	c.JSON(http.StatusOK, updated)
}

func (h *WorkerHandler) DeleteWorker(c *gin.Context) {
	wid, err := id.FromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid worker ID"})
		return
	}
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userIDObj, err := id.FromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	stopped := false
	if h.connectionMgr != nil {
		if conn := h.connectionMgr.Get(wid.Hex()); conn != nil {
			conn.Shutdown()
			stopped = true
		}
	}

	if err := h.workerService.Delete(c.Request.Context(), wid, userIDObj); err != nil {
		if err.Error() == "worker not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "unauthorized: worker does not belong to user" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	msg := "Worker deleted successfully"
	if stopped {
		msg = "Worker deleted; stop signal sent to live agent"
	}
	c.JSON(http.StatusOK, gin.H{"message": msg, "stop_signal_sent": stopped})
}
