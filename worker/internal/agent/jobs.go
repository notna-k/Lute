package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	pb "github.com/lute/proto"

	"github.com/lute/worker/internal/joblog"
	"github.com/lute/worker/internal/runner"
)

func execute(ctx context.Context, jobLogsDir string, a *pb.JobAssignment) error {
	slog.Info("Executing job", "job_id", a.JobId, "type", a.Type, "payload_size", len(a.Payload), "timeout_sec", a.TimeoutSec)
	switch a.Type {
	case "noop":
		return nil
	case "container":
		var spec runner.Spec
		if err := json.Unmarshal(a.Payload, &spec); err != nil {
			return fmt.Errorf("decode container spec: %w", err)
		}
		return runner.Run(ctx, a.JobId, jobLogsDir, &spec, a.TimeoutSec)
	default:
		return fmt.Errorf("no handler registered for job type %q", a.Type)
	}
}

func readJobLog(jobLogsDir string, req *pb.JobLogRequest) *pb.JobLogResponse {
	resp := &pb.JobLogResponse{RequestId: req.RequestId}
	if jobLogsDir == "" {
		resp.Error = "job logs directory not configured on worker"
		return resp
	}
	path, err := joblog.Path(jobLogsDir, req.JobId)
	if err != nil {
		resp.Error = err.Error()
		return resp
	}

	var r joblog.Result
	if req.GetDirection() == pb.LogReadDirection_LOG_READ_HEAD {
		r = joblog.ReadHead(path, int(req.Limit), req.AnchorOffset)
	} else {
		r = joblog.ReadTail(path, int(req.Limit), req.AnchorOffset)
	}
	resp.Lines = r.Lines
	resp.NextAnchor = r.NextAnchor
	resp.FileSize = r.FileSize
	resp.HasMore = r.HasMore
	resp.Error = r.Err
	return resp
}
