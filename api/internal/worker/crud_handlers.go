package worker

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/httpx"
	"github.com/lute/api/internal/version"
)

type patchLabelsRequest struct {
	Labels map[string]string `json:"labels" binding:"required"`
}

// ownedWorker loads the :id worker if it belongs to the caller. A foreign worker is
// reported as missing, so its existence is not revealed.
func (h *WorkerHandler) ownedWorker(c *gin.Context) (*models.Worker, bool) {
	wid, err := id.FromHex(c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid worker id")
		return nil, false
	}
	userID, ok := httpx.UserID(c)
	if !ok {
		return nil, false
	}
	w, err := h.workerRepo.GetByID(c.Request.Context(), wid)
	if errors.Is(err, repos.ErrNotFound) || (err == nil && w.UserID != userID) {
		httpx.Error(c, http.StatusNotFound, "worker not found")
		return nil, false
	}
	if err != nil {
		httpx.Internal(c, err)
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
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := models.ValidateWorkerLabels(req.Labels); err != nil {
		httpx.Invalid(c, err.Error(), map[string]string{"labels": err.Error()})
		return
	}

	ctx := c.Request.Context()
	if err := h.workerRepo.UpdateLabels(ctx, w.ID, req.Labels); err != nil {
		httpx.Internal(c, err)
		return
	}
	updated, err := h.workerRepo.GetByID(ctx, w.ID)
	if err != nil {
		httpx.Internal(c, err)
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

func (h *WorkerHandler) GetWorker(c *gin.Context) {
	if w, ok := h.ownedWorker(c); ok {
		c.JSON(http.StatusOK, withOutdated(w))
	}
}

func (h *WorkerHandler) ListUserWorkers(c *gin.Context) {
	userID, ok := httpx.UserID(c)
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
		httpx.Internal(c, err)
		return
	}
	for _, w := range list {
		withOutdated(w)
	}
	c.JSON(http.StatusOK, list)
}

// withOutdated flags an agent older than core, so the panel can ask for an image pull.
func withOutdated(w *models.Worker) *models.Worker {
	w.Outdated = version.Older(w.AgentVersion, version.Version)
	return w
}

type updateWorkerRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// UpdateWorker renames or re-describes a worker. Only those columns are written: the
// secret, status and what the agent reported belong to core.
func (h *WorkerHandler) UpdateWorker(c *gin.Context) {
	w, ok := h.ownedWorker(c)
	if !ok {
		return
	}
	var req updateWorkerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	updates := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if err := models.ValidateWorkerName(name); err != nil {
			httpx.Invalid(c, err.Error(), map[string]string{"name": err.Error()})
			return
		}
		updates["name"] = name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	ctx := c.Request.Context()
	if err := h.workerRepo.UpdateFields(ctx, w.ID, updates); err != nil {
		if errors.Is(err, repos.ErrDuplicate) {
			httpx.Error(c, http.StatusConflict, "a worker with that name already exists")
			return
		}
		httpx.Internal(c, err)
		return
	}
	updated, err := h.workerRepo.GetByID(ctx, w.ID)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, withOutdated(updated))
}

// ReEnableWorker moves a dead worker back to pending so its agent may reconnect.
func (h *WorkerHandler) ReEnableWorker(c *gin.Context) {
	w, ok := h.ownedWorker(c)
	if !ok {
		return
	}
	if w.Status != enums.WorkerDead {
		httpx.Error(c, http.StatusBadRequest, "worker is not dead; only dead workers can be re-enabled")
		return
	}
	ctx := c.Request.Context()
	if err := h.workerRepo.UpdateStatus(ctx, w.ID, enums.WorkerPending); err != nil {
		httpx.Internal(c, err)
		return
	}
	updated, err := h.workerRepo.GetByID(ctx, w.ID)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

// DeleteWorker drains a connected worker: it finishes its running jobs, reports drained,
// and core then removes the row while the agent stops its own container. A worker that
// is not connected is removed at once; the lease reaper fails whatever it was running.
func (h *WorkerHandler) DeleteWorker(c *gin.Context) {
	w, ok := h.ownedWorker(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	if conn := h.connectionMgr.Get(w.ID.Hex()); conn != nil {
		if err := h.workerRepo.UpdateStatus(ctx, w.ID, enums.WorkerDeleting); err != nil {
			httpx.Internal(c, err)
			return
		}
		// Look again: a reconnect may have replaced the stream since; its Connect reads
		// the status after publishing, so one of the two always signals the live stream.
		if current := h.connectionMgr.Get(w.ID.Hex()); current != nil {
			conn = current
		}
		conn.Shutdown()
		c.JSON(http.StatusAccepted, gin.H{"status": enums.WorkerDeleting, "message": "The worker finishes its running jobs, then stops."})
		return
	}
	if err := h.workerRepo.Delete(ctx, w.ID); err != nil {
		httpx.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *WorkerHandler) GetWorkerLiveStatus(c *gin.Context) {
	worker, ok := h.ownedWorker(c)
	if !ok {
		return
	}
	result := gin.H{
		"worker_id": worker.ID.Hex(),
		"name":      worker.Name,
		"status":    worker.Status,
	}
	if worker.Status != enums.WorkerPending && !worker.LastSeen.IsZero() {
		result["agent_version"] = worker.AgentVersion
		result["last_seen"] = worker.LastSeen
		result["metrics"] = worker.Metrics
	}
	c.JSON(http.StatusOK, result)
}

func (h *WorkerHandler) ListConnectedWorkers(c *gin.Context) {
	w := h.connectionMgr.ActiveWorkers()
	c.JSON(http.StatusOK, gin.H{"workers": w, "count": len(w)})
}
