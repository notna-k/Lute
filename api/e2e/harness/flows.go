//go:build e2e

package harness

import (
	"fmt"
	"sync/atomic"
	"time"
)

// The flows here are the multi-step sequences a real operator performs, kept in one
// place so a test reads as the scenario it is about rather than as plumbing.

var ipSeq atomic.Uint32

// NextAgentIP hands out a distinct address per registration. Core refuses a second
// live worker at one address, which is correct — and which means tests that want
// several workers must look like several machines.
func NextAgentIP() string {
	return fmt.Sprintf("10.90.%d.%d", ipSeq.Add(1)/250%250, ipSeq.Load()%250+1)
}

// ClaimWorker performs the claim-code half of onboarding: the operator asks the panel
// for a code, and the host registers itself with it. Returns core's answer.
func (s *Stack) ClaimWorker(c *Client, name string) Registered {
	s.t.Helper()

	code, err := c.CreateClaimCode()
	if err != nil {
		s.t.Fatalf("create claim code: %v", err)
	}
	reg, err := c.RegisterWorker(WorkerRegistration{
		Name:      name,
		Hostname:  name + ".e2e",
		OS:        "linux",
		Arch:      "amd64",
		CPUs:      2,
		IP:        NextAgentIP(),
		Version:   "e2e",
		ClaimCode: code.Code,
	})
	if err != nil {
		s.t.Fatalf("register worker %s: %v", name, err)
	}
	return reg
}

// ConnectedAgent onboards a host and starts its agent, waiting until core reports the
// stream as live. This is the state most scenarios need before they begin.
func (s *Stack) ConnectedAgent(c *Client, name string, options ...AgentOption) *Agent {
	s.t.Helper()

	reg := s.ClaimWorker(c, name)
	agent := s.StartAgent(reg.WorkerID, options...)
	s.WaitConnected(c, reg.WorkerID)
	return agent
}

// WaitConnected blocks until core lists the worker as holding a live stream.
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

// WaitDisconnected blocks until core no longer holds a stream for the worker.
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

// WaitAllDisconnected blocks until core holds no agent streams at all.
func (s *Stack) WaitAllDisconnected(c *Client) {
	s.t.Helper()
	WaitFor(s.t, 30*time.Second, "every agent to disconnect", func() bool {
		workers, err := c.ConnectedWorkers()
		return err == nil && len(workers) == 0
	})
}

// WaitBuildStatus blocks until a build reaches one of the wanted statuses and returns it.
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

// WaitJobStatus blocks until the queue reports one of the wanted statuses for a job.
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

// WaitExecution blocks until core has recorded a finished attempt for a job.
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

// WaitLogLine blocks until a build's log contains a line matching want.
func WaitLogLine(c *Client, jobID string, timeout time.Duration, contains func(string) bool) []string {
	return Eventually(c.t, timeout, "a matching line in the log of "+jobID,
		func() ([]string, bool) {
			page, err := c.JobLogs(jobID, LogOptions{Limit: 200})
			if err != nil {
				return nil, false
			}
			for _, line := range page.Lines {
				if contains(line) {
					return page.Lines, true
				}
			}
			return page.Lines, false
		})
}
