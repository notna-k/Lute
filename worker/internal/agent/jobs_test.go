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
	if err := execute(ctx, "", &pb.JobAssignment{JobId: "1", Type: "noop"}); err != nil {
		t.Errorf("noop: %v", err)
	}
	err := execute(ctx, "", &pb.JobAssignment{JobId: "2", Type: "teleport"})
	if err == nil || !strings.Contains(err.Error(), `"teleport"`) {
		t.Errorf("unknown type: err = %v", err)
	}
	err = execute(ctx, "", &pb.JobAssignment{JobId: "3", Type: "container", Payload: []byte("{")})
	if err == nil || !strings.Contains(err.Error(), "decode container spec") {
		t.Errorf("bad payload: err = %v", err)
	}
}

func TestReadJobLog(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "job-42.log"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		dir  string
		req  *pb.JobLogRequest
		want *pb.JobLogResponse
	}{
		{
			name: "tail",
			dir:  dir,
			req:  &pb.JobLogRequest{RequestId: "r1", JobId: "42", Limit: 2},
			want: &pb.JobLogResponse{RequestId: "r1", Lines: []string{"two", "three"}, NextAnchor: 4, FileSize: 14, HasMore: true},
		},
		{
			name: "head",
			dir:  dir,
			req:  &pb.JobLogRequest{RequestId: "r2", JobId: "42", Limit: 1, Direction: pb.LogReadDirection_LOG_READ_HEAD},
			want: &pb.JobLogResponse{RequestId: "r2", Lines: []string{"one"}, NextAnchor: 4, FileSize: 14, HasMore: true},
		},
		{
			name: "no logs dir",
			req:  &pb.JobLogRequest{RequestId: "r3", JobId: "42"},
			want: &pb.JobLogResponse{RequestId: "r3", Error: "job logs directory not configured on worker"},
		},
		{
			name: "escaping job id",
			dir:  dir,
			req:  &pb.JobLogRequest{RequestId: "r4", JobId: "x/../../secret"},
			want: &pb.JobLogResponse{RequestId: "r4", Error: `invalid job id "x/../../secret"`},
		},
		{
			name: "missing file",
			dir:  dir,
			req:  &pb.JobLogRequest{RequestId: "r5", JobId: "7"},
			want: &pb.JobLogResponse{RequestId: "r5", Error: "log file not found"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readJobLog(tt.dir, tt.req)
			if !proto.Equal(tt.want, got) {
				t.Errorf("got  %v\nwant %v", got, tt.want)
			}
		})
	}
}
