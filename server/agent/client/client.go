package client

import (
	"context"
	"log"
	"time"

	pb "github.com/lute/agent/proto/agent"
)

// UpdateStatus updates the machine status on the server
func UpdateStatus(client pb.AgentServiceClient, agentID, machineID, status, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.UpdateMachineStatus(ctx, &pb.UpdateMachineStatusRequest{
		AgentId:   agentID,
		MachineId: machineID,
		Status:    status,
		Message:   message,
	})
	if err != nil {
		log.Printf("UpdateMachineStatus(%s) failed: %v", status, err)
		return
	}
	log.Printf("UpdateMachineStatus(%s): %s", status, resp.Message)
}

// Register registers the agent with the server
func Register(ctx context.Context, client pb.AgentServiceClient, agentID, machineID, version, ipAddress string, metadata map[string]string) (*pb.RegisterAgentResponse, error) {
	resp, err := client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		AgentId:   agentID,
		MachineId: machineID,
		Version:   version,
		IpAddress: ipAddress,
		Metadata:  metadata,
	})
	return resp, err
}
