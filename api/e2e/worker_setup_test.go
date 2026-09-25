//go:build e2e

package e2e

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/lute/api/e2e/harness"
)

// TestWorkerOnboarding covers claiming a build host: the panel issues a code, the host
// registers with it, and core refuses everything that is not exactly that.
func TestWorkerOnboarding(t *testing.T) {
	stack := newBareStack(t)
	admin := stack.AdminClient()

	t.Run("Success - a claimed host appears under the account that claimed it", func(t *testing.T) {
		code, err := admin.CreateClaimCode()
		if err != nil {
			t.Fatalf("create claim code: %v", err)
		}
		if code.Code == "" {
			t.Fatal("no claim code issued")
		}

		reg, err := admin.RegisterWorker(harness.WorkerRegistration{
			Name:      "builder-one",
			Hostname:  "builder-one.e2e",
			OS:        "linux",
			Arch:      "amd64",
			CPUs:      4,
			IP:        harness.NextAgentIP(),
			Version:   "e2e",
			ClaimCode: code.Code,
		})
		if err != nil {
			t.Fatalf("register: %v", err)
		}
		if reg.WorkerID == "" {
			t.Fatal("registration returned no worker id")
		}
		// The address core hands back is the one the agent will dial. If it is not
		// reachable, onboarding looks successful and the host never connects.
		if reg.GRPCAddress == "" {
			t.Fatal("registration returned no gRPC address")
		}
		conn, err := net.DialTimeout("tcp", reg.GRPCAddress, 5*time.Second)
		if err != nil {
			t.Fatalf("the gRPC address core returned (%s) is not reachable: %v", reg.GRPCAddress, err)
		}
		_ = conn.Close()

		workers, err := admin.ListWorkers()
		if err != nil {
			t.Fatalf("list workers: %v", err)
		}
		var found *harness.Worker
		for i := range workers {
			if workers[i].ID == reg.WorkerID {
				found = &workers[i]
			}
		}
		if found == nil {
			t.Fatalf("worker %s is not listed for the account that claimed it", reg.WorkerID)
		}
		if found.Name != "builder-one" {
			t.Errorf("name = %q, want builder-one", found.Name)
		}
		if found.Status != "registered" {
			t.Errorf("status = %q, want registered", found.Status)
		}
	})

	t.Run("Fail - no claim code; the host is not registered", func(t *testing.T) {
		_, err := admin.RegisterWorker(harness.WorkerRegistration{
			Name: "no-code",
			IP:   harness.NextAgentIP(),
		})
		if harness.StatusOf(err) != http.StatusBadRequest {
			t.Fatalf("register without a claim code: err = %v, want 400", err)
		}
		assertNoWorkerNamed(t, admin, "no-code")
	})

	t.Run("Fail - a made-up claim code is refused", func(t *testing.T) {
		_, err := admin.RegisterWorker(harness.WorkerRegistration{
			Name:      "forged",
			IP:        harness.NextAgentIP(),
			ClaimCode: "0123456789abcdef0123456789abcdef",
		})
		if harness.StatusOf(err) != http.StatusBadRequest {
			t.Fatalf("register with a forged code: err = %v, want 400", err)
		}
		assertNoWorkerNamed(t, admin, "forged")
	})

	t.Run("Fail - a claim code is single use", func(t *testing.T) {
		code, err := admin.CreateClaimCode()
		if err != nil {
			t.Fatalf("create claim code: %v", err)
		}
		if _, err := admin.RegisterWorker(harness.WorkerRegistration{
			Name:      "reuse-first",
			IP:        harness.NextAgentIP(),
			ClaimCode: code.Code,
		}); err != nil {
			t.Fatalf("first registration: %v", err)
		}

		// A code that works twice lets anyone who sees it enrol their own host.
		_, err = admin.RegisterWorker(harness.WorkerRegistration{
			Name:      "reuse-second",
			IP:        harness.NextAgentIP(),
			ClaimCode: code.Code,
		})
		if harness.StatusOf(err) != http.StatusBadRequest {
			t.Fatalf("reusing a claim code: err = %v, want 400", err)
		}
		assertNoWorkerNamed(t, admin, "reuse-second")
	})

	t.Run("Fail - a second live host at one address is refused", func(t *testing.T) {
		ip := harness.NextAgentIP()

		code, _ := admin.CreateClaimCode()
		if _, err := admin.RegisterWorker(harness.WorkerRegistration{
			Name: "twin-first", IP: ip, ClaimCode: code.Code,
		}); err != nil {
			t.Fatalf("first host at %s: %v", ip, err)
		}

		second, _ := admin.CreateClaimCode()
		_, err := admin.RegisterWorker(harness.WorkerRegistration{
			Name: "twin-second", IP: ip, ClaimCode: second.Code,
		})
		if harness.StatusOf(err) != http.StatusConflict {
			t.Fatalf("second host at the same address: err = %v, want 409", err)
		}
	})

	t.Run("Success - a dead host's address can be claimed again", func(t *testing.T) {
		ip := harness.NextAgentIP()

		code, _ := admin.CreateClaimCode()
		first, err := admin.RegisterWorker(harness.WorkerRegistration{
			Name: "recycled-first", IP: ip, ClaimCode: code.Code,
		})
		if err != nil {
			t.Fatalf("first host: %v", err)
		}

		// A host that was replaced or reimaged has to be able to come back. Deleting
		// the old record is the operator's way of saying so.
		if err := admin.DeleteWorker(first.WorkerID); err != nil {
			t.Fatalf("delete first host: %v", err)
		}

		second, _ := admin.CreateClaimCode()
		replacement, err := admin.RegisterWorker(harness.WorkerRegistration{
			Name: "recycled-second", IP: ip, ClaimCode: second.Code,
		})
		if err != nil {
			t.Fatalf("re-registering at %s after the old host was deleted: %v", ip, err)
		}
		if replacement.WorkerID == first.WorkerID {
			t.Error("the replacement reused the deleted host's id")
		}
	})
}

// TestWorkerSetupCommand runs the agent's own `setup` subcommand, the path an operator
// actually follows: paste the command from the panel, answer the prompt, and the host
// registers itself and starts working.
func TestWorkerSetupCommand(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()

	code, err := admin.CreateClaimCode()
	if err != nil {
		t.Fatalf("create claim code: %v", err)
	}

	setup := stack.RunSetupCLI(code.Code, "setup-host")

	if setup.WorkerID == "" {
		t.Fatalf("setup printed no worker id:\n%s", setup.Output)
	}

	workers, err := admin.ListWorkers()
	if err != nil {
		t.Fatalf("list workers: %v", err)
	}
	found := false
	for _, w := range workers {
		if w.ID == setup.WorkerID {
			found = true
			if w.Name != "setup-host" {
				t.Errorf("name = %q, want the name typed at the prompt", w.Name)
			}
		}
	}
	if !found {
		t.Fatalf("the host setup registered (%s) is not listed", setup.WorkerID)
	}

	// setup does not stop at registering: it starts the agent, so the host is ready
	// to take work without a second command.
	stack.WaitConnected(admin, setup.WorkerID)
}

func assertNoWorkerNamed(t *testing.T, c *harness.Client, name string) {
	t.Helper()
	workers, err := c.ListWorkers()
	if err != nil {
		t.Fatalf("list workers: %v", err)
	}
	for _, w := range workers {
		if w.Name == name {
			t.Fatalf("worker %q was registered and should not have been", name)
		}
	}
}
