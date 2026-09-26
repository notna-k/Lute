//go:build e2e

package harness

import (
	"fmt"
	"time"
)

// Registered is a worker enrolled through the Register RPC, as an agent would.
type Registered struct {
	WorkerID string
	Secret   string
}

// RegisterWorker enrols name with the bootstrap token over gRPC, for a RawWorker to use.
func (s *Stack) RegisterWorker(name string) Registered {
	s.t.Helper()
	resp, err := s.Register(BootstrapToken, name)
	if err != nil {
		s.t.Fatalf("register worker %s: %v", name, err)
	}
	return Registered{WorkerID: resp.WorkerId, Secret: resp.Secret}
}

// ConnectedAgent starts an agent and returns once it has registered and its stream is live.
func (s *Stack) ConnectedAgent(c *Client, name string, options ...AgentOption) *Agent {
	s.t.Helper()
	agent := s.StartAgent(name, options...)
	agent.WorkerID = s.WaitWorkerNamed(c, agent).ID
	s.WaitConnected(c, agent.WorkerID)
	return agent
}

// WaitWorkerNamed waits for the worker an agent registers under its name.
func (s *Stack) WaitWorkerNamed(c *Client, agent *Agent) Worker {
	s.t.Helper()
	return Eventually(s.t, 30*time.Second, "a worker named "+agent.Name+" to register", func() (Worker, bool) {
		if agent.Exited() {
			s.t.Fatalf("agent %s exited before registering (%v):\n%s", agent.Name, agent.ExitError(), agent.Stderr())
		}
		workers, err := c.ListWorkers()
		if err != nil {
			return Worker{}, false
		}
		for _, w := range workers {
			if w.Name == agent.Name {
				return w, true
			}
		}
		return Worker{}, false
	})
}

func (s *Stack) WaitConnected(c *Client, workerID string) ConnectedWorker {
	s.t.Helper()
	return Eventually(s.t, 30*time.Second, "worker "+workerID+" to connect",
		func() (ConnectedWorker, bool) {
			workers, err := c.ConnectedWorkers()
			if err != nil {
				return ConnectedWorker{}, false
			}
			for _, w := range workers {
				if w.WorkerID == workerID {
					return w, true
				}
			}
			return ConnectedWorker{}, false
		})
}

func (s *Stack) WaitDisconnected(c *Client, workerID string) {
	s.t.Helper()
	WaitFor(s.t, 30*time.Second, "worker "+workerID+" to disconnect", func() bool {
		workers, err := c.ConnectedWorkers()
		if err != nil {
			return false
		}
		for _, w := range workers {
			if w.WorkerID == workerID {
				return false
			}
		}
		return true
	})
}

func (s *Stack) WaitAllDisconnected(c *Client) {
	s.t.Helper()
	WaitFor(s.t, 30*time.Second, "every agent to disconnect", func() bool {
		workers, err := c.ConnectedWorkers()
		return err == nil && len(workers) == 0
	})
}

func WaitBuildStatus(c *Client, slug, runID string, timeout time.Duration, wanted ...string) Build {
	return Eventually(c.t, timeout,
		fmt.Sprintf("build %s of %s to reach %v", runID, slug, wanted),
		func() (Build, bool) {
			builds, err := c.Builds(slug)
			if err != nil {
				return Build{}, false
			}
			for _, b := range builds {
				if b.RunID != runID {
					continue
				}
				for _, w := range wanted {
					if b.Status == w {
						return b, true
					}
				}
				return b, false
			}
			return Build{}, false
		})
}

func WaitJobStatus(c *Client, jobID string, timeout time.Duration, wanted ...string) Job {
	return Eventually(c.t, timeout,
		fmt.Sprintf("job %s to reach %v", jobID, wanted),
		func() (Job, bool) {
			job, err := c.GetJob(jobID)
			if err != nil {
				return Job{}, false
			}
			for _, w := range wanted {
				if job.Status == w {
					return job, true
				}
			}
			return job, false
		})
}

func WaitExecution(c *Client, jobID string, timeout time.Duration) Execution {
	return Eventually(c.t, timeout, "an execution record for job "+jobID,
		func() (Execution, bool) {
			execs, err := c.Executions()
			if err != nil {
				return Execution{}, false
			}
			for _, e := range execs {
				if e.JobID == jobID {
					return e, true
				}
			}
			return Execution{}, false
		})
}
