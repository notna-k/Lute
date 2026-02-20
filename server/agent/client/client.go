package client

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/lute/agent/proto/agent"
)

// UpdateStatus updates the machine status on the server
func UpdateStatus(client pb.AgentServiceClient, machineID, status, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.UpdateMachineStatus(ctx, &pb.UpdateMachineStatusRequest{
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
// If machineID is empty, server will create a new machine
// Returns the machine_id (either provided or newly created)
func Register(ctx context.Context, client pb.AgentServiceClient, machineID, version, ipAddress, hostname string, metadata map[string]string) (string, error) {
	resp, err := client.RegisterAgent(ctx, &pb.RegisterAgentRequest{
		MachineId: machineID,
		Version:   version,
		IpAddress: ipAddress,
		Hostname:  hostname,
		Metadata:  metadata,
	})
	if err != nil {
		return "", err
	}
	if !resp.Success {
		return "", fmt.Errorf("%s", resp.Message)
	}
	return resp.MachineId, nil
}
