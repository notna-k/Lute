package config

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/lute/agent/commands"
	"github.com/lute/agent/types"

	pb "github.com/lute/agent/proto/agent"
)

// PollLoop periodically fetches config and executes pending commands
func PollLoop(ctx context.Context, client pb.AgentServiceClient, agentID, machineID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Config poll started (every %s)", interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Config poll stopped")
			return
		case <-ticker.C:
			pollConfig(ctx, client, agentID, machineID)
		}
	}
}

// pollConfig fetches config from server and applies it
func pollConfig(ctx context.Context, client pb.AgentServiceClient, agentID, machineID string) {
	pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := client.GetMachineConfig(pollCtx, &pb.GetMachineConfigRequest{
		AgentId:   agentID,
		MachineId: machineID,
	})
	if err != nil {
		log.Printf("GetMachineConfig failed: %v", err)
		return
	}

	if !resp.Success {
		log.Printf("GetMachineConfig: server returned failure: %s", resp.Message)
		return
	}

	// Apply config updates
	Apply(resp.Config)

	// Execute any pending commands
	executePendingCommands(ctx, client, agentID, machineID, resp.Config)
}

// Apply applies server-pushed configuration
func Apply(cfg map[string]string) {
	if lvl, ok := cfg["log_level"]; ok {
		log.Printf("Config: log_level=%s", lvl)
	}
	// heartbeat_interval changes would require restarting the ticker;
	// for now we just log it.
	if hb, ok := cfg["heartbeat_interval"]; ok {
		log.Printf("Config: heartbeat_interval=%ss", hb)
	}
}

// executePendingCommands parses and executes pending commands from config
func executePendingCommands(ctx context.Context, client pb.AgentServiceClient, agentID, machineID string, cfg map[string]string) {
	raw, ok := cfg["pending_commands"]
	if !ok || raw == "" {
		return
	}

	var cmds []types.PendingCmd
	if err := json.Unmarshal([]byte(raw), &cmds); err != nil {
		log.Printf("Failed to parse pending_commands: %v", err)
		return
	}

	for _, cmd := range cmds {
		go commands.Execute(ctx, client, agentID, machineID, commands.Cmd{
			ID:      cmd.ID,
			Command: cmd.Command,
			Args:    cmd.Args,
		})
	}
}
