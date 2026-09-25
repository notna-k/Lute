package worker

import (
	"testing"

	pb "github.com/lute/proto"
)

func metric(v float64) *pb.MetricValue { return &pb.MetricValue{Kind: &pb.MetricValue_F{F: v}} }

func TestMetricsOfKeepsCanonicalFloats(t *testing.T) {
	got := metricsOf(map[string]*pb.MetricValue{
		"cpu_load":      metric(0.5),
		"mem_usage_mb":  metric(512),
		"hostname":      metric(1),                          // not a metric the panel charts
		"disk_used_gb":  {Kind: &pb.MetricValue_S{S: "12"}}, // not a float
		"disk_total_gb": nil,
	})
	if len(got) != 2 || got["cpu_load"] != 0.5 || got["mem_usage_mb"] != 512.0 {
		t.Fatalf("metrics = %v, want cpu_load and mem_usage_mb only", got)
	}
}

func TestMetricsOfIsNilWithoutMetrics(t *testing.T) {
	var pong *pb.HeartbeatPong
	if got := metricsOf(pong.GetMetrics()); got != nil {
		t.Fatalf("metrics = %v, want nil", got)
	}
	if got := metricsOf(map[string]*pb.MetricValue{"hostname": metric(1)}); got != nil {
		t.Fatalf("metrics = %v, want nil", got)
	}
}
