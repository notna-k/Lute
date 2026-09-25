package worker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
)

var labelKeyRe = regexp.MustCompile(`^[a-zA-Z0-9_\-.]{1,63}$`)

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

func currentUserID(c *gin.Context) (id.ID, bool) {
	raw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return "", false
	}
	uid, err := id.FromHex(raw.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return "", false
	}
	return uid, true
}

// ownedWorker loads the :id worker if it belongs to the caller. A foreign worker is
// reported as missing, so its existence is not revealed.
func (h *WorkerHandler) ownedWorker(c *gin.Context) (*models.Worker, bool) {
	wid, err := id.FromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid worker ID"})
		return nil, false
	}
	userID, ok := currentUserID(c)
	if !ok {
		return nil, false
	}
	w, err := h.workerRepo.GetByID(c.Request.Context(), wid)
	if errors.Is(err, repos.ErrNotFound) || (err == nil && w.UserID != userID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "worker not found"})
		return nil, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return nil, false
	}
	return w, true
}

func (h *WorkerHandler) GetLabels(c *gin.Context) {
	w, ok := h.ownedWorker(c)
	if !ok {
		return
	}
	labels := w.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	c.JSON(http.StatusOK, gin.H{"labels": labels})
}

func (h *WorkerHandler) PatchLabels(c *gin.Context) {
	w, ok := h.ownedWorker(c)
	if !ok {
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

	w.Labels = req.Labels
	updated, err := h.save(c.Request.Context(), w)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Keep in-memory dispatch state current and re-evaluate jobs waiting on a selector.
	h.connectionMgr.UpdateWorkerLabels(w.ID.Hex(), req.Labels)
	if conn := h.connectionMgr.Get(w.ID.Hex()); conn != nil {
		for _, q := range conn.Queues {
			h.grpcServer.DispatchQueue(context.Background(), q)
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
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	w.UserID = userID
	if w.Status == "" {
		w.Status = enums.WorkerPending
	}
	if err := h.workerRepo.Create(c.Request.Context(), &w); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, &w)
}

func (h *WorkerHandler) GetWorker(c *gin.Context) {
	if w, ok := h.ownedWorker(c); ok {
		c.JSON(http.StatusOK, w)
	}
}

func (h *WorkerHandler) ListUserWorkers(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	filter := map[string]string{}
	for _, lv := range c.QueryArray("label") {
		if k, v, ok := strings.Cut(lv, ":"); ok {
			filter[k] = v
		}
	}
	list, err := h.workerRepo.GetByUserIDAndLabels(c.Request.Context(), userID, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *WorkerHandler) UpdateWorker(c *gin.Context) {
	existing, ok := h.ownedWorker(c)
	if !ok {
		return
	}
	var w models.Worker
	if err := c.ShouldBindJSON(&w); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	w.ID, w.UserID = existing.ID, existing.UserID
	updated, err := h.save(c.Request.Context(), &w)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// ReEnableWorker moves a dead worker back to pending so its agent may reconnect.
func (h *WorkerHandler) ReEnableWorker(c *gin.Context) {
	w, ok := h.ownedWorker(c)
	if !ok {
		return
	}
	if w.Status != enums.WorkerDead {
		c.JSON(http.StatusBadRequest, gin.H{"error": "worker is not dead; only dead workers can be re-enabled"})
		return
	}
	ctx := c.Request.Context()
	if err := h.workerRepo.UpdateStatus(ctx, w.ID, enums.WorkerPending); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	updated, err := h.workerRepo.GetByID(ctx, w.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *WorkerHandler) DeleteWorker(c *gin.Context) {
	w, ok := h.ownedWorker(c)
	if !ok {
		return
	}
	stopped := false
	if conn := h.connectionMgr.Get(w.ID.Hex()); conn != nil {
		conn.Shutdown()
		stopped = true
	}
	if err := h.workerRepo.Delete(c.Request.Context(), w.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	msg := "Worker deleted successfully"
	if stopped {
		msg = "Worker deleted; stop signal sent to live agent"
	}
	c.JSON(http.StatusOK, gin.H{"message": msg, "stop_signal_sent": stopped})
}

func (h *WorkerHandler) save(ctx context.Context, w *models.Worker) (*models.Worker, error) {
	if err := h.workerRepo.Update(ctx, w.ID, w); err != nil {
		return nil, err
	}
	return h.workerRepo.GetByID(ctx, w.ID)
}
