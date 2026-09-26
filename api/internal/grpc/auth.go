package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/version"
	"github.com/lute/api/internal/workerauth"
	pb "github.com/lute/proto"
)

// Register enrols an agent that presents a registration token and gives it a secret.
func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if err := s.checkProtocol(req.GetProtocol()); err != nil {
		return nil, err
	}
	token, err := s.tokenRepo.GetActiveByHash(ctx, workerauth.Hash(req.GetToken()))
	if errors.Is(err, repos.ErrNotFound) {
		return nil, status.Error(codes.Unauthenticated, "registration token is invalid or revoked; create one under Workers → Tokens")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "look up token: %v", err)
	}

	name := strings.TrimSpace(req.GetName())
	if err := models.ValidateWorkerName(name); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := models.ValidateWorkerLabels(req.GetLabels()); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	secret, err := workerauth.NewSecret()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate secret: %v", err)
	}
	eng := req.GetEngine()
	w := &models.Worker{
		UserID:       token.CreatedBy,
		Name:         name,
		Description:  fmt.Sprintf("Registered with token %q", token.Name),
		Status:       enums.WorkerRegistered,
		AgentVersion: req.GetVersion(),
		Protocol:     req.GetProtocol(),
		Engine:       engineOf(eng),
		SecretHash:   workerauth.Hash(secret),
		Labels:       req.GetLabels(),
		Metadata: map[string]any{
			"hostname": eng.GetHostname(),
			"os":       eng.GetOs(),
			"arch":     eng.GetArch(),
			"cpus":     eng.GetCpus(),
			"ip":       peerIP(ctx),
		},
	}
	if err := s.workerRepo.Create(ctx, w); err != nil {
		if errors.Is(err, repos.ErrDuplicate) {
			return nil, status.Errorf(codes.AlreadyExists, "a worker named %q already exists; set LUTE_NAME to a free name", name)
		}
		return nil, status.Errorf(codes.Internal, "create worker: %v", err)
	}
	if err := s.tokenRepo.TouchUsed(ctx, token.ID); err != nil {
		slog.Warn("touch registration token", "token_id", token.ID.Hex(), "err", err)
	}
	slog.Info("worker registered", "worker_id", w.ID.Hex(), "name", name, "token", token.Name)
	return &pb.RegisterResponse{WorkerId: w.ID.Hex(), Secret: secret}, nil
}

// authenticate resolves the worker a Connect stream's bearer credential names. A deleted
// worker is NotFound, so its agent knows to stop instead of retrying.
func (s *Server) authenticate(ctx context.Context) (*models.Worker, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	values := md.Get("authorization")
	if len(values) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization metadata")
	}
	workerID, secret, err := workerauth.ParseBearer(values[0])
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	wid, err := ParseWorkerID(workerID)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "malformed worker id")
	}
	w, err := s.workerRepo.GetByID(ctx, wid)
	if errors.Is(err, repos.ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "worker %s was deleted", workerID)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "look up worker: %v", err)
	}
	if !workerauth.Matches(secret, w.SecretHash) {
		return nil, status.Error(codes.Unauthenticated, "wrong worker secret")
	}
	return w, nil
}

func (s *Server) checkProtocol(p int32) error {
	if p >= pb.MinProtocol {
		return nil
	}
	return status.Errorf(codes.FailedPrecondition,
		"agent protocol %d is older than the minimum %d; pull %s and recreate the container",
		p, pb.MinProtocol, s.WorkerImage())
}

// WorkerImage is the agent image operators should run against this core.
func (s *Server) WorkerImage() string {
	if s.config.Workers.Image != "" {
		return s.config.Workers.Image
	}
	return "ghcr.io/notna-k/lute-worker:" + version.MinorTag(version.Version)
}

func engineOf(e *pb.EngineInfo) *models.Engine {
	if e == nil {
		return nil
	}
	return &models.Engine{
		Kind:        e.GetKind(),
		Version:     e.GetVersion(),
		Rootless:    e.GetRootless(),
		MemoryLimit: e.GetMemoryLimit(),
		CPULimit:    e.GetCpuLimit(),
		PidsLimit:   e.GetPidsLimit(),
	}
}

func peerIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return p.Addr.String()
	}
	return host
}
