package worker

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
)

type WorkerService struct {
	workerRepo *repos.WorkerRepository
}

func NewWorkerService(workerRepo *repos.WorkerRepository) *WorkerService {
	return &WorkerService{workerRepo: workerRepo}
}

func (s *WorkerService) Create(ctx context.Context, userID primitive.ObjectID, w *models.Worker) (*models.Worker, error) {
	w.UserID = userID
	if w.Status == "" {
		w.Status = "pending"
	}
	if err := s.workerRepo.Create(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *WorkerService) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Worker, error) {
	w, err := s.workerRepo.GetByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("worker not found")
		}
		return nil, err
	}
	return w, nil
}

func (s *WorkerService) GetByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.Worker, error) {
	return s.workerRepo.GetByUserID(ctx, userID)
}

func (s *WorkerService) GetPublic(ctx context.Context) ([]*models.Worker, error) {
	return s.workerRepo.GetPublic(ctx)
}

func (s *WorkerService) Update(ctx context.Context, id primitive.ObjectID, userID primitive.ObjectID, w *models.Worker) (*models.Worker, error) {
	existing, err := s.workerRepo.GetByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("worker not found")
		}
		return nil, err
	}
	if existing.UserID != userID {
		return nil, errors.New("unauthorized: worker does not belong to user")
	}
	w.UserID = existing.UserID
	w.ID = existing.ID
	if err := s.workerRepo.Update(ctx, id, w); err != nil {
		return nil, err
	}
	return s.workerRepo.GetByID(ctx, id)
}

func (s *WorkerService) Delete(ctx context.Context, id primitive.ObjectID, userID primitive.ObjectID) error {
	existing, err := s.workerRepo.GetByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("worker not found")
		}
		return err
	}
	if existing.UserID != userID {
		return errors.New("unauthorized: worker does not belong to user")
	}
	return s.workerRepo.Delete(ctx, id)
}

func (s *WorkerService) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	return s.workerRepo.UpdateStatus(ctx, id, status)
}

func (s *WorkerService) FindByAgentID(ctx context.Context, agentID string) (*models.Worker, error) {
	return s.workerRepo.FindByAgentID(ctx, agentID)
}
