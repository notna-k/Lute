package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/lute/proto"
)

func TestExecute(t *testing.T) {
	ctx := context.Background()
	j := &Jobs{}
	if err := j.Execute(ctx, &pb.JobAssignment{JobId: "1", Type: "noop"}); err != nil {
		t.Errorf("noop: %v", err)
	}
	err := j.Execute(ctx, &pb.JobAssignment{JobId: "2", Type: "teleport"})
	if err == nil || !strings.Contains(err.Error(), `"teleport"`) {
		t.Errorf("unknown type: err = %v", err)
	}
	err = j.Execute(ctx, &pb.JobAssignment{JobId: "3", Type: "container", Payload: []byte("{")})
	if err == nil || !strings.Contains(err.Error(), "decode container spec") {
		t.Errorf("bad payload: err = %v", err)
	}
}

func TestReadLog(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "jobs", "42"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "jobs", "42", "log"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	j := &Jobs{DataDir: root}

	tests := []struct {
		name string
		req  *pb.JobLogRequest
		want *pb.JobLogResponse
	}{
		{
			name: "tail",
			req:  &pb.JobLogRequest{RequestId: "r1", JobId: "42", Limit: 2},
			want: &pb.JobLogResponse{RequestId: "r1", Lines: []string{"two", "three"}, NextAnchor: 4, FileSize: 14, HasMore: true},
		},
		{
			name: "head",
			req:  &pb.JobLogRequest{RequestId: "r2", JobId: "42", Limit: 1, Direction: pb.LogReadDirection_LOG_READ_HEAD},
			want: &pb.JobLogResponse{RequestId: "r2", Lines: []string{"one"}, NextAnchor: 4, FileSize: 14, HasMore: true},
		},
		{
			name: "escaping job id",
			req:  &pb.JobLogRequest{RequestId: "r4", JobId: "x/../../secret"},
			want: &pb.JobLogResponse{RequestId: "r4", Error: `invalid job id "x/../../secret"`},
		},
		{
			name: "missing file",
			req:  &pb.JobLogRequest{RequestId: "r5", JobId: "7"},
			want: &pb.JobLogResponse{RequestId: "r5", Error: "log file not found"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := j.ReadLog(tt.req)
			if !proto.Equal(tt.want, got) {
				t.Errorf("got  %v\nwant %v", got, tt.want)
			}
		})
	}
}

func TestFinishJobWithoutStreamKeepsResult(t *testing.T) {
	a := New(Config{}, context.Background())
	a.running["job-1"] = struct{}{}

	a.finishJob(&pb.JobResult{JobId: "job-1", Success: true}) // no stream: the result is queued

	if _, ok := a.running["job-1"]; ok {
		t.Error("finished job is still counted as running")
	}
	if len(a.pending) != 1 || a.pending[0].JobId != "job-1" {
		t.Errorf("pending = %v, want the result kept for the next stream", a.pending)
	}
}

func TestDrainWaitsForPendingResults(t *testing.T) {
	a := New(Config{}, context.Background())
	stopped := false
	a.stop = func() { stopped = true }
	a.pending = []*pb.JobResult{{JobId: "job-1"}}

	a.finishGracefully(Drained)

	if _, done := a.finished(); done || stopped {
		t.Fatal("agent stopped with a result nobody received")
	}
	if a.finishing == nil || *a.finishing != Drained {
		t.Errorf("finishing = %v, want Drained once the result is sent", a.finishing)
	}
}
