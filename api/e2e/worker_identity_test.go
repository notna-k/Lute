//go:build e2e

package e2e

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/lute/api/e2e/harness"
)

// TestWorkerRegistration covers enrolling with a registration token and keeping the identity.
func TestWorkerRegistration(t *testing.T) {
	stack := newBareStack(t)
	admin := stack.AdminClient()

	t.Run("Success - an agent registers from its environment and keeps its id across restarts", func(t *testing.T) {
		agent := stack.ConnectedAgent(admin, "env-only", harness.WithQueues("build"))

		w, err := admin.GetWorker(agent.WorkerID)
		if err != nil {
			t.Fatalf("get worker: %v", err)
		}
		if w.Engine == nil || w.Engine.Kind != "docker" {
			t.Errorf("engine = %+v, want what the agent probed", w.Engine)
		}
		if w.Protocol == 0 {
			t.Error("the agent's protocol was not recorded")
		}
		state, err := os.ReadFile(filepath.Join(agent.DataDir, "state.json"))
		if err != nil {
			t.Fatalf("no state.json in the data dir: %v", err)
		}
		if strings.Contains(string(state), harness.BootstrapToken) {
			t.Error("state.json holds the registration token; it only needs the worker secret")
		}

		agent.Kill()
		stack.WaitDisconnected(admin, agent.WorkerID)
		again := agent.Restart()
		stack.WaitConnected(admin, agent.WorkerID)

		// A restart that registered again would leave a second row, or fail on the taken name.
		workers, err := admin.ListWorkers()
		if err != nil {
			t.Fatalf("list workers: %v", err)
		}
		named := 0
		for _, w := range workers {
			if w.Name == "env-only" {
				named++
			}
		}
		if named != 1 {
			t.Errorf("%d workers named env-only after a restart, want 1", named)
		}
		if again.Exited() {
			t.Fatalf("the restarted agent exited:\n%s", again.Stderr())
		}
	})

	t.Run("Success - a token made in the panel enrols a worker and records its use", func(t *testing.T) {
		tok, err := admin.CreateToken("build-farm")
		if err != nil {
			t.Fatalf("create token: %v", err)
		}
		if !strings.HasPrefix(tok.Token, "lute_rt_") || !strings.HasPrefix(tok.Token, tok.Prefix) {
			t.Fatalf("token %q, prefix %q", tok.Token, tok.Prefix)
		}
		stack.ConnectedAgent(admin, "farm-01", harness.WithVar("LUTE_TOKEN", tok.Token))

		tokens, err := admin.ListTokens()
		if err != nil {
			t.Fatalf("list tokens: %v", err)
		}
		for _, lt := range tokens {
			if lt.ID != tok.ID {
				continue
			}
			if lt.Token != "" {
				t.Error("the token list exposes the plaintext")
			}
			if lt.LastUsedAt == "" {
				t.Error("last_used_at was not set by the registration")
			}
			return
		}
		t.Fatalf("token %s is not listed", tok.ID)
	})

	t.Run("Success - revoking a token stops new workers but not registered ones", func(t *testing.T) {
		tok, err := admin.CreateToken("short-lived")
		if err != nil {
			t.Fatalf("create token: %v", err)
		}
		enrolled := stack.ConnectedAgent(admin, "before-revoke", harness.WithVar("LUTE_TOKEN", tok.Token))

		if err := admin.RevokeToken(tok.ID); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		late := stack.StartAgent("after-revoke", harness.WithVar("LUTE_TOKEN", tok.Token))
		late.WaitExit(30 * time.Second)
		if late.ExitCode() != 78 {
			t.Errorf("exit code = %d, want 78 (a person must fix the token)", late.ExitCode())
		}
		if !strings.Contains(late.Stderr(), "invalid or revoked") {
			t.Errorf("the agent did not say why:\n%s", late.Stderr())
		}
		assertNoWorkerNamed(t, admin, "after-revoke")

		enrolled.Kill()
		stack.WaitDisconnected(admin, enrolled.WorkerID)
		enrolled.Restart()
		stack.WaitConnected(admin, enrolled.WorkerID)
	})

	t.Run("Fail - a second worker cannot take a name", func(t *testing.T) {
		stack.ConnectedAgent(admin, "taken-name")

		twin := stack.StartAgent("taken-name")
		twin.WaitExit(30 * time.Second)
		if twin.ExitCode() != 78 {
			t.Errorf("exit code = %d, want 78", twin.ExitCode())
		}
		if !strings.Contains(twin.Stderr(), "LUTE_NAME") {
			t.Errorf("the agent did not point at LUTE_NAME:\n%s", twin.Stderr())
		}

		_, err := stack.Register(harness.BootstrapToken, "taken-name")
		if status.Code(err) != codes.AlreadyExists {
			t.Errorf("Register with a taken name: %v, want AlreadyExists", err)
		}
	})

	t.Run("Fail - an agent without a token or identity does not start", func(t *testing.T) {
		agent := stack.StartAgent("no-token", harness.WithVar("LUTE_TOKEN", ""))
		agent.WaitExit(30 * time.Second)
		if agent.ExitCode() != 78 || !strings.Contains(agent.Stderr(), "LUTE_TOKEN") {
			t.Errorf("exit %d, stderr:\n%s", agent.ExitCode(), agent.Stderr())
		}
	})

	t.Run("Fail - a rootful engine is refused unless allowed", func(t *testing.T) {
		if harness.Rootless() {
			t.Skip("the engine is rootless")
		}
		agent := stack.StartAgent("rootful-host", harness.WithVar("LUTE_ALLOW_ROOTFUL", ""))
		agent.WaitExit(30 * time.Second)
		if agent.ExitCode() != 78 {
			t.Errorf("exit code = %d, want 78", agent.ExitCode())
		}
		if !strings.Contains(agent.Stderr(), "rootful") {
			t.Errorf("the agent did not say why:\n%s", agent.Stderr())
		}
		assertNoWorkerNamed(t, admin, "rootful-host")
	})

	t.Run("Fail - a data dir that belongs to something else is refused", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("mine"), 0o600); err != nil {
			t.Fatal(err)
		}
		agent := stack.StartAgent("wrong-mount", harness.WithDataDir(dir))
		agent.WaitExit(30 * time.Second)
		if agent.ExitCode() != 78 || !strings.Contains(agent.Stderr(), "marker") {
			t.Errorf("exit %d, stderr:\n%s", agent.ExitCode(), agent.Stderr())
		}
	})

	t.Run("Success - the Add Worker dialog gets an address and image", func(t *testing.T) {
		info, err := admin.InstallInfo()
		if err != nil {
			t.Fatalf("install info: %v", err)
		}
		if !strings.HasSuffix(info.Server, ":"+stack.Config.GRPC.Port) {
			t.Errorf("server = %q, want the gRPC port %s", info.Server, stack.Config.GRPC.Port)
		}
		if !strings.Contains(info.Image, "lute-worker") {
			t.Errorf("image = %q", info.Image)
		}
	})
}

// TestWorkerCredentials: only the secret core issued lets a stream in.
func TestWorkerCredentials(t *testing.T) {
	stack := newBareStack(t)
	admin := stack.AdminClient()
	reg := stack.RegisterWorker("credentialed")

	t.Run("Fail - a made-up token cannot register", func(t *testing.T) {
		_, err := stack.Register("lute_rt_made_up", "forger")
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("err = %v, want Unauthenticated", err)
		}
		assertNoWorkerNamed(t, admin, "forger")
	})

	t.Run("Fail - a real worker id with a forged secret is refused", func(t *testing.T) {
		raw, err := stack.DialRawWorker(reg.WorkerID, "lute_ws_forged")
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if err := raw.Register(1, "build"); err != nil {
			t.Logf("send: %v", err)
		}
		if code := status.Code(raw.WaitForStreamEnd(15 * time.Second)); code != codes.Unauthenticated {
			t.Errorf("stream ended with %v, want Unauthenticated", code)
		}
	})

	t.Run("Fail - a stream without a registration first is refused", func(t *testing.T) {
		raw, err := stack.DialRawWorker(reg.WorkerID, reg.Secret)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if err := raw.ReportResult("x", true, "", 1); err != nil {
			t.Logf("send: %v", err)
		}
		if code := status.Code(raw.WaitForStreamEnd(15 * time.Second)); code != codes.InvalidArgument {
			t.Errorf("stream ended with %v, want InvalidArgument", code)
		}
	})

	t.Run("Fail - an agent speaking an old protocol is told to update", func(t *testing.T) {
		raw, err := stack.DialRawWorker(reg.WorkerID, reg.Secret)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if err := raw.RegisterWithProtocol(1); err != nil {
			t.Fatalf("register: %v", err)
		}
		err = raw.WaitForStreamEnd(15 * time.Second)
		if status.Code(err) != codes.FailedPrecondition || !strings.Contains(err.Error(), "lute-worker") {
			t.Errorf("stream ended with %v, want FailedPrecondition naming the image to pull", err)
		}
	})

	t.Run("Success - the right secret connects", func(t *testing.T) {
		raw, err := stack.DialRawWorker(reg.WorkerID, reg.Secret)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if err := raw.Register(1, "build"); err != nil {
			t.Fatalf("register: %v", err)
		}
		stack.WaitConnected(admin, reg.WorkerID)
	})

	t.Run("Fail - token management needs a signed-in operator", func(t *testing.T) {
		if _, err := stack.Client().ListTokens(); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Errorf("anonymous list: err = %v, want 401", err)
		}
	})
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
