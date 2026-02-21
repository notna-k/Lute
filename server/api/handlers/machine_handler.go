package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/lute/api/models"
	"github.com/lute/api/services"
)

type MachineHandler struct {
	machineService *services.MachineService
}

func NewMachineHandler(machineService *services.MachineService) *MachineHandler {
	return &MachineHandler{
		machineService: machineService,
	}
}

// CreateMachine handles POST /api/v1/machines
func (h *MachineHandler) CreateMachine(c *gin.Context) {
	var machine models.Machine
	if err := c.ShouldBindJSON(&machine); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDObj, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	createdMachine, err := h.machineService.Create(c.Request.Context(), userIDObj, &machine)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, createdMachine)
}

// GetMachine handles GET /api/v1/machines/:id
func (h *MachineHandler) GetMachine(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid machine ID"})
		return
	}

	machine, err := h.machineService.GetByID(c.Request.Context(), id)
	if err != nil {
		if err.Error() == "machine not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, machine)
}

// ListUserMachines handles GET /api/v1/machines
func (h *MachineHandler) ListUserMachines(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDObj, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	machines, err := h.machineService.GetByUserID(c.Request.Context(), userIDObj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, machines)
}

// ListPublicMachines handles GET /api/v1/machines/public
func (h *MachineHandler) ListPublicMachines(c *gin.Context) {
	machines, err := h.machineService.GetPublic(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, machines)
}

// UpdateMachine handles PUT /api/v1/machines/:id
func (h *MachineHandler) UpdateMachine(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid machine ID"})
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDObj, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var machine models.Machine
	if err := c.ShouldBindJSON(&machine); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedMachine, err := h.machineService.Update(c.Request.Context(), id, userIDObj, &machine)
	if err != nil {
		if err.Error() == "machine not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "unauthorized: machine does not belong to user" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedMachine)
}

// ReEnableMachine handles POST /api/v1/machines/:id/re-enable (only when status is "dead").
func (h *MachineHandler) ReEnableMachine(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid machine ID"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDObj, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	existing, err := h.machineService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
		return
	}
	if existing.UserID != userIDObj {
		c.JSON(http.StatusForbidden, gin.H{"error": "machine not found"})
		return
	}
	if existing.Status != "dead" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "machine is not dead; only dead machines can be re-enabled"})
		return
	}

	if err := h.machineService.UpdateStatus(c.Request.Context(), id, "pending"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updated, _ := h.machineService.GetByID(c.Request.Context(), id)
	c.JSON(http.StatusOK, updated)
}

// DeleteMachine handles DELETE /api/v1/machines/:id
func (h *MachineHandler) DeleteMachine(c *gin.Context) {
	id, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid machine ID"})
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userIDObj, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	if err := h.machineService.Delete(c.Request.Context(), id, userIDObj); err != nil {
		if err.Error() == "machine not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err.Error() == "unauthorized: machine does not belong to user" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Machine deleted successfully"})
}
