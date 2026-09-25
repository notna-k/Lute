package worker

import (
	"sync"

	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/repos"
	luteGrpc "github.com/lute/api/internal/grpc"
)

type WorkerHandler struct {
	binaryDir     string
	binaryMu      sync.RWMutex // guards binaryCache
	binaryCache   map[string]*WorkerBinaryInfo
	claimMu       sync.RWMutex // guards claimCodes
	claimCodes    map[string]*claimEntry
	cfg           *config.Config
	workerRepo    *repos.WorkerRepository
	commandRepo   *repos.CommandRepository
	connectionMgr *luteGrpc.ConnectionManager
	grpcServer    *luteGrpc.Server
}

func NewWorkerHandler(cfg *config.Config, workerRepo *repos.WorkerRepository, commandRepo *repos.CommandRepository, grpcServer *luteGrpc.Server) *WorkerHandler {
	handler := &WorkerHandler{
		binaryDir:     cfg.WorkerBinary.Dir,
		binaryCache:   make(map[string]*WorkerBinaryInfo),
		claimCodes:    make(map[string]*claimEntry),
		cfg:           cfg,
		workerRepo:    workerRepo,
		commandRepo:   commandRepo,
		connectionMgr: grpcServer.ConnMgr,
		grpcServer:    grpcServer,
	}
	handler.refreshBinaryCache()
	return handler
}
