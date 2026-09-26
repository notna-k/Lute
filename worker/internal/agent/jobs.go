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

// Jobs runs assignments with a runner and reads logs from the data dir.
type Jobs struct {
	Runner  *runner.Runner
	DataDir string
}

func (j *Jobs) Execute(ctx context.Context, a *pb.JobAssignment) error {
	slog.Info("Executing job", "job_id", a.JobId, "type", a.Type, "payload_size", len(a.Payload), "timeout_sec", a.TimeoutSec)
	switch a.Type {
	case "noop":
		return nil
	case "container":
		var spec runner.Spec
		if err := json.Unmarshal(a.Payload, &spec); err != nil {
			return fmt.Errorf("decode container spec: %w", err)
		}
		return j.Runner.Run(ctx, a.JobId, &spec, a.TimeoutSec)
	default:
		return fmt.Errorf("no handler registered for job type %q", a.Type)
	}
}

func (j *Jobs) ReadLog(req *pb.JobLogRequest) *pb.JobLogResponse {
	resp := &pb.JobLogResponse{RequestId: req.RequestId}
	path, err := joblog.Path(j.DataDir, req.JobId)
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
