package services

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/lute/api/models"
	"github.com/lute/api/repository"
)

type MachineService struct {
	machineRepo *repository.MachineRepository
}

func NewMachineService(machineRepo *repository.MachineRepository) *MachineService {
	return &MachineService{
		machineRepo: machineRepo,
	}
}

// Create creates a new machine
func (s *MachineService) Create(ctx context.Context, userID primitive.ObjectID, machine *models.Machine) (*models.Machine, error) {
	// Set user ID and default status
	machine.UserID = userID
	if machine.Status == "" {
		machine.Status = "pending"
	}

	if err := s.machineRepo.Create(ctx, machine); err != nil {
		return nil, err
	}

	return machine, nil
}

// GetByID retrieves a machine by ID
func (s *MachineService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Machine, error) {
	machine, err := s.machineRepo.GetByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("machine not found")
		}
		return nil, err
	}
	return machine, nil
}

// GetByUserID retrieves all machines for a user
func (s *MachineService) GetByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.Machine, error) {
	return s.machineRepo.GetByUserID(ctx, userID)
}

// GetPublic retrieves all public machines
func (s *MachineService) GetPublic(ctx context.Context) ([]*models.Machine, error) {
	return s.machineRepo.GetPublic(ctx)
}

// Update updates an existing machine
func (s *MachineService) Update(ctx context.Context, id primitive.ObjectID, userID primitive.ObjectID, machine *models.Machine) (*models.Machine, error) {
	// Verify machine exists and belongs to user
	existing, err := s.machineRepo.GetByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("machine not found")
		}
		return nil, err
	}

	// Verify ownership
	if existing.UserID != userID {
		return nil, errors.New("unauthorized: machine does not belong to user")
	}

	// Preserve user ID and ID
	machine.UserID = existing.UserID
	machine.ID = existing.ID

	if err := s.machineRepo.Update(ctx, id, machine); err != nil {
		return nil, err
	}

	// Return updated machine
	return s.machineRepo.GetByID(ctx, id)
}

// Delete deletes a machine
func (s *MachineService) Delete(ctx context.Context, id primitive.ObjectID, userID primitive.ObjectID) error {
	// Verify machine exists and belongs to user
	existing, err := s.machineRepo.GetByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("machine not found")
		}
		return err
	}

	// Verify ownership
	if existing.UserID != userID {
		return errors.New("unauthorized: machine does not belong to user")
	}

	return s.machineRepo.Delete(ctx, id)
}

// UpdateStatus updates the status of a machine
func (s *MachineService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	return s.machineRepo.UpdateStatus(ctx, id, status)
}

// FindByAgentID finds a machine by agent ID
func (s *MachineService) FindByAgentID(ctx context.Context, agentID string) (*models.Machine, error) {
	return s.machineRepo.FindByAgentID(ctx, agentID)
}
