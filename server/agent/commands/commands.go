package commands

import (
	"context"
	"log"
	"os/exec"
	"strconv"
	"time"

	pb "github.com/lute/agent/proto/agent"
)

// Execute runs a command locally and reports the result back to the server
func Execute(ctx context.Context, client pb.AgentServiceClient, machineID string, cmd Cmd) {
	log.Printf("Executing command %s: %s %v", cmd.ID, cmd.Command, cmd.Args)

	// Notify server that we're picking up this command
	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, _ = client.ExecuteCommand(execCtx, &pb.ExecuteCommandRequest{
		MachineId: machineID,
		Command:   cmd.Command,
		Args:      cmd.Args,
		Env:       map[string]string{"command_id": cmd.ID, "stage": "start"},
	})

	// Execute locally
	c := exec.CommandContext(execCtx, cmd.Command, cmd.Args...)
	output, err := c.CombinedOutput()

	exitCode := 0
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// Report result back
	_, reportErr := client.ExecuteCommand(execCtx, &pb.ExecuteCommandRequest{
		MachineId: machineID,
		Command:   cmd.Command,
		Args:      cmd.Args,
		Env: map[string]string{
			"command_id": cmd.ID,
			"stage":      "done",
			"output":     string(output),
			"exit_code":  strconv.Itoa(exitCode),
			"error":      errMsg,
		},
	})
	if reportErr != nil {
		log.Printf("Failed to report command result for %s: %v", cmd.ID, reportErr)
	} else {
		log.Printf("Command %s finished: exit=%d", cmd.ID, exitCode)
	}
}

// Cmd represents a command to execute
type Cmd struct {
	ID      string
	Command string
	Args    []string
}

