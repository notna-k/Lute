package worker

import (
	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/repos"
	luteGrpc "github.com/lute/api/internal/grpc"
)

type WorkerHandler struct {
	cfg           *config.Config
	workerRepo    *repos.WorkerRepository
	tokenRepo     *repos.RegistrationTokenRepository
	commandRepo   *repos.CommandRepository
	connectionMgr *luteGrpc.ConnectionManager
	grpcServer    *luteGrpc.Server
}

func NewWorkerHandler(cfg *config.Config, workerRepo *repos.WorkerRepository, tokenRepo *repos.RegistrationTokenRepository, commandRepo *repos.CommandRepository, grpcServer *luteGrpc.Server) *WorkerHandler {
	return &WorkerHandler{
		cfg:           cfg,
		workerRepo:    workerRepo,
		tokenRepo:     tokenRepo,
		commandRepo:   commandRepo,
		connectionMgr: grpcServer.ConnMgr,
		grpcServer:    grpcServer,
	}
}
