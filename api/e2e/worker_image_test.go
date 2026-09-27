//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/lute/api/e2e/harness"
)

// TestWorkerImage runs the built image, as operators do. make e2e-image builds it and sets
// LUTE_E2E_WORKER_IMAGE; the plain suite skips this test.
func TestWorkerImage(t *testing.T) {
	image := os.Getenv("LUTE_E2E_WORKER_IMAGE")
	if image == "" {
		t.Skip("LUTE_E2E_WORKER_IMAGE is not set; run make e2e-image")
	}

	t.Run("Fail - without the data dir mounted the container exits 78", func(t *testing.T) {
		out, err := exec.Command("docker", "run", "--rm", image).CombinedOutput()
		var exit *exec.ExitError
		if !asExit(err, &exit) || exit.ExitCode() != 78 {
			t.Fatalf("exit = %v, want 78:\n%s", err, out)
		}
		if !strings.Contains(string(out), "is not mounted") {
			t.Errorf("the container did not say why:\n%s", out)
		}
	})

	t.Run("Success - a job runs, and a deleted worker's container stays stopped", func(t *testing.T) {
		stack := newStack(t)
		admin := stack.AdminClient()

		name := "image-worker"
		volume := "lute-e2e-" + strings.ToLower(harness.SafeName(t.Name()))
		dockerOK(t, "volume", "create", volume)
		t.Cleanup(func() {
			_ = exec.Command("docker", "rm", "-f", name).Run()
			_ = exec.Command("docker", "volume", "rm", "-f", volume).Run()
		})
		dockerOK(t, "run", "-d", "--name", name, "--restart", "unless-stopped",
			"--network", "host",
			"-v", "/var/run/docker.sock:/var/run/docker.sock",
			"-v", volume+":/var/lib/lute-worker",
			"-e", "LUTE_SERVER="+stack.GRPCAddr(),
			"-e", "LUTE_TOKEN="+harness.BootstrapToken,
			"-e", "LUTE_NAME="+name,
			"-e", "LUTE_QUEUES=build",
			"-e", "LUTE_ALLOW_ROOTFUL=1",
			image)
		t.Cleanup(func() {
			if t.Failed() {
				out, _ := exec.Command("docker", "logs", name).CombinedOutput()
				t.Logf("container log:\n%s", out)
			}
		})

		worker := harness.Eventually(t, time.Minute, "the container to register", func() (harness.Worker, bool) {
			workers, err := admin.ListWorkers()
			if err != nil {
				return harness.Worker{}, false
			}
			for _, w := range workers {
				if w.Name == name {
					return w, true
				}
			}
			return harness.Worker{}, false
		})
		stack.WaitConnected(admin, worker.ID)

		enqueued, err := admin.Enqueue(harness.EnqueueRequest{
			Queue: "build", Type: "container", TimeoutSec: 120,
			Payload: containerPayload(t, "bash:5", "echo from-a-sibling; sleep 3"),
		})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		harness.WaitJobStatus(admin, enqueued.JobID, time.Minute, "running")

		if _, err := admin.DeleteWorker(worker.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		harness.WaitJobStatus(admin, enqueued.JobID, time.Minute, "done")
		harness.WaitFor(t, time.Minute, "the container to stop itself", func() bool {
			return containerState(name) == "exited 0"
		})
		// Docker restarts an exited unless-stopped container; one stopped through the API stays down.
		harness.Never(t, 5*time.Second, "Docker to restart the deleted worker", func() bool {
			return containerState(name) != "exited 0"
		})
		if _, err := admin.GetWorker(worker.ID); harness.StatusOf(err) != 404 {
			t.Errorf("get deleted worker: err = %v, want 404", err)
		}
	})
}

func containerState(name string) string {
	out, err := exec.Command("docker", "inspect", "-f", "{{.State.Status}} {{.RestartCount}}", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func dockerOK(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command("docker", args...).CombinedOutput(); err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func asExit(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}
