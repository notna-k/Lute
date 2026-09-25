package worker

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/httpx"
)

type SendCommandRequest struct {
	Command string            `json:"command" binding:"required"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

func (h *WorkerHandler) SendCommand(c *gin.Context) {
	worker, ok := h.ownedWorker(c)
	if !ok {
		return
	}
	var req SendCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if worker.Status == "" || worker.Status == enums.WorkerPending {
		httpx.Error(c, http.StatusBadRequest, "worker has no active agent connection")
		return
	}

	cmd := &models.Command{
		WorkerID: worker.ID,
		Command:  req.Command,
		Args:     req.Args,
		Env:      req.Env,
		Status:   enums.CommandPending,
	}
	if err := h.commandRepo.Create(c.Request.Context(), cmd); err != nil {
		httpx.Internal(c, fmt.Errorf("queue command: %w", err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"command_id": cmd.ID.Hex(),
		"status":     enums.CommandPending,
		"message":    "Command queued for worker",
	})
}

func (h *WorkerHandler) ListCommands(c *gin.Context) {
	worker, ok := h.ownedWorker(c)
	if !ok {
		return
	}
	commands, err := h.commandRepo.GetByWorkerID(c.Request.Context(), worker.ID, 50)
	if err != nil {
		httpx.Internal(c, fmt.Errorf("list commands: %w", err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"commands": commands, "count": len(commands)})
}

func (h *WorkerHandler) GetCommandResult(c *gin.Context) {
	cmdID, err := id.FromHex(c.Param("commandId"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid command_id")
		return
	}
	userID, ok := httpx.UserID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	cmd, err := h.commandRepo.GetByID(ctx, cmdID)
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "command not found")
		return
	}
	if w, err := h.workerRepo.GetByID(ctx, cmd.WorkerID); err != nil || w.UserID != userID {
		httpx.Error(c, http.StatusNotFound, "command not found")
		return
	}
	c.JSON(http.StatusOK, cmd)
}
