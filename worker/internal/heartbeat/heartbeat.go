package heartbeat

import (
	"fmt"
	"time"

	pb "github.com/lute/proto"

	"github.com/lute/worker/internal/metrics"
)

// PongMessage builds a WorkerMessage containing a HeartbeatPong with current
// status and collected metrics. Caller is responsible for sending it on the stream.
func PongMessage(machineID string) *pb.WorkerMessage {
	raw := metrics.Collect()
	pong := &pb.HeartbeatPong{
		Status:    "running",
		Metrics:   metricsToProto(raw),
		Timestamp: time.Now().Unix(),
	}
	return &pb.WorkerMessage{
		MachineId: machineID,
		Payload:   &pb.WorkerMessage_HeartbeatPong{HeartbeatPong: pong},
	}
}

func metricsToProto(raw map[string]interface{}) map[string]*pb.MetricValue {
	out := make(map[string]*pb.MetricValue, len(raw))
	for k, v := range raw {
		out[k] = toMetricValue(v)
	}
	return out
}

func toMetricValue(v interface{}) *pb.MetricValue {
	switch x := v.(type) {
	case int64:
		return &pb.MetricValue{Kind: &pb.MetricValue_I{I: x}}
	case int:
		return &pb.MetricValue{Kind: &pb.MetricValue_I{I: int64(x)}}
	case float64:
		return &pb.MetricValue{Kind: &pb.MetricValue_F{F: x}}
	case string:
		return &pb.MetricValue{Kind: &pb.MetricValue_S{S: x}}
	default:
		return &pb.MetricValue{Kind: &pb.MetricValue_S{S: fmt.Sprint(v)}}
	}
}
