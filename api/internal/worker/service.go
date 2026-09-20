package worker

import (
	"context"
	"errors"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
)

type WorkerService struct {
	workerRepo *repos.WorkerRepository
}

func NewWorkerService(workerRepo *repos.WorkerRepository) *WorkerService {
	return &WorkerService{workerRepo: workerRepo}
}

func (s *WorkerService) Create(ctx context.Context, userID id.ID, w *models.Worker) (*models.Worker, error) {
	w.UserID = userID
	if w.Status == "" {
		w.Status = "pending"
	}
	if err := s.workerRepo.Create(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *WorkerService) GetByID(ctx context.Context, wid id.ID) (*models.Worker, error) {
	w, err := s.workerRepo.GetByID(ctx, wid)
	if err != nil {
		if errors.Is(err, repos.ErrNotFound) {
			return nil, errors.New("worker not found")
		}
		return nil, err
	}
	return w, nil
}

func (s *WorkerService) GetByUserID(ctx context.Context, userID id.ID) ([]*models.Worker, error) {
	return s.workerRepo.GetByUserID(ctx, userID)
}

func (s *WorkerService) Update(ctx context.Context, wid id.ID, userID id.ID, w *models.Worker) (*models.Worker, error) {
	existing, err := s.workerRepo.GetByID(ctx, wid)
	if err != nil {
		if errors.Is(err, repos.ErrNotFound) {
			return nil, errors.New("worker not found")
		}
		return nil, err
	}
	if existing.UserID != userID {
		return nil, errors.New("unauthorized: worker does not belong to user")
	}
	w.UserID = existing.UserID
	w.ID = existing.ID
	if err := s.workerRepo.Update(ctx, wid, w); err != nil {
		return nil, err
	}
	return s.workerRepo.GetByID(ctx, wid)
}

func (s *WorkerService) Delete(ctx context.Context, wid id.ID, userID id.ID) error {
	existing, err := s.workerRepo.GetByID(ctx, wid)
	if err != nil {
		if errors.Is(err, repos.ErrNotFound) {
			return errors.New("worker not found")
		}
		return err
	}
	if existing.UserID != userID {
		return errors.New("unauthorized: worker does not belong to user")
	}
	return s.workerRepo.Delete(ctx, wid)
}

func (s *WorkerService) UpdateStatus(ctx context.Context, wid id.ID, status string) error {
	return s.workerRepo.UpdateStatus(ctx, wid, status)
}

func (s *WorkerService) FindByAgentID(ctx context.Context, agentID string) (*models.Worker, error) {
	return s.workerRepo.FindByAgentID(ctx, agentID)
}
