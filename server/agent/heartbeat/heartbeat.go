package heartbeat

import (
	"context"
	"log"
	"time"

	"github.com/lute/agent/metrics"

	pb "github.com/lute/agent/proto/agent"
)

// Loop sends periodic heartbeats with status and system metrics
func Loop(ctx context.Context, client pb.AgentServiceClient, agentID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Heartbeat loop started (every %s)", interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Heartbeat loop stopped")
			return
		case <-ticker.C:
			sendHeartbeat(ctx, client, agentID)
		}
	}
}

// sendHeartbeat sends a single heartbeat to the server
func sendHeartbeat(ctx context.Context, client pb.AgentServiceClient, agentID string) {
	hbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := client.Heartbeat(hbCtx, &pb.HeartbeatRequest{
		AgentId: agentID,
		Status:  "running",
		Metrics: metrics.Collect(),
	})

	if err != nil {
		log.Printf("Heartbeat failed: %v", err)
	}
}

// SendFinal sends a final heartbeat before shutdown
func SendFinal(ctx context.Context, client pb.AgentServiceClient, agentID string) {
	hbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, _ = client.Heartbeat(hbCtx, &pb.HeartbeatRequest{
		AgentId: agentID,
		Status:  "stopped",
	})
}
