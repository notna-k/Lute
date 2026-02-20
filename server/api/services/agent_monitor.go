package services

import (
	"context"
	"log"
	"time"

	"github.com/lute/api/repository"
)

// AgentMonitor periodically checks for dead agents
type AgentMonitor struct {
	machineRepo *repository.MachineRepository
	interval    time.Duration
	timeout     time.Duration
}

// NewAgentMonitor creates a new agent monitor
func NewAgentMonitor(
	machineRepo *repository.MachineRepository,
	checkInterval time.Duration,
	heartbeatTimeout time.Duration,
) *AgentMonitor {
	return &AgentMonitor{
		machineRepo: machineRepo,
		interval:    checkInterval,
		timeout:     heartbeatTimeout,
	}
}

// Start begins monitoring agents in the background
func (m *AgentMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	log.Printf("Agent monitor started (check every %s, timeout %s)", m.interval, m.timeout)

	for {
		select {
		case <-ctx.Done():
			log.Println("Agent monitor stopped")
			return
		case <-ticker.C:
			m.checkDeadAgents(ctx)
		}
	}
}

// checkDeadAgents finds machines that haven't sent heartbeat and marks them as dead
func (m *AgentMonitor) checkDeadAgents(ctx context.Context) {
	// Find all machines that are currently "alive"
	aliveMachines, err := m.machineRepo.ListByStatus(ctx, "alive")
	if err != nil {
		log.Printf("Agent monitor: failed to list alive machines: %v", err)
		return
	}

	now := time.Now()
	deadCount := 0

	for _, machine := range aliveMachines {
		// Skip machines that don't have agent info (never connected)
		if machine.LastSeen.IsZero() {
			continue
		}

		// Check if last_seen is older than timeout
		timeSinceLastSeen := now.Sub(machine.LastSeen)
		if timeSinceLastSeen > m.timeout {
			// Mark machine as dead
			if err := m.machineRepo.UpdateStatus(ctx, machine.ID, "dead"); err != nil {
				log.Printf("Agent monitor: failed to mark machine %s as dead: %v", machine.ID.Hex(), err)
				continue
			}

			log.Printf("Agent monitor: marked machine %s as dead (last seen %s ago)", 
				machine.ID.Hex(), timeSinceLastSeen)
			deadCount++
		}
	}

	if deadCount > 0 {
		log.Printf("Agent monitor: marked %d machine(s) as dead", deadCount)
	}
}
