//go:build e2e

package e2e

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lute/api/e2e/harness"
)

// TestPublicRunsAPI covers the integration surface: API keys, runs, status and logs.
func TestPublicRunsAPI(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()
	stack.ConnectedAgent(admin, "public-host", harness.WithQueues("build"))

	key, err := admin.CreateAPIKey("integration")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	if key.Token == "" {
		t.Fatal("the created key carries no token; it can never be used")
	}
	api := admin.WithAPIKey(key.Token)

	t.Run("Success - a run created with a key executes and reports its outcome", func(t *testing.T) {
		run, err := api.CreateRun(harness.CreateRunRequest{
			Queue:   "build",
			Type:    "container",
			Payload: containerPayload(t, "bash:5", `echo "hello from the public api"`),
		})
		if err != nil {
			t.Fatalf("create run: %v", err)
		}
		if run.ID == "" {
			t.Fatal("create returned no run id")
		}

		done := harness.Eventually(t, 2*time.Minute, "the run to finish",
			func() (harness.Run, bool) {
				got, err := api.GetRun(run.ID)
				if err != nil {
					return got, false
				}
				return got, got.Status == "done" || got.Status == "dead"
			})
		if done.Status != "done" {
			t.Fatalf("run finished %q, want done: %s", done.Status, done.Error)
		}
		if done.WorkerID == "" {
			t.Error("the finished run does not say which host ran it")
		}

		logs, err := api.RunLogs(run.ID, harness.LogOptions{Limit: 100})
		if err != nil {
			t.Fatalf("read run logs: %v", err)
		}
		if !linesContain(logs.Lines, "hello from the public api") {
			t.Errorf("the run's output is missing:\n%s", strings.Join(logs.Lines, "\n"))
		}
	})

	t.Run("Success - the same idempotency key returns the original run", func(t *testing.T) {
		req := harness.CreateRunRequest{
			Queue:          "build",
			Type:           "noop",
			IdempotencyKey: "deploy-2026-09-24-001",
		}

		first, err := api.CreateRun(req)
		if err != nil {
			t.Fatalf("first create: %v", err)
		}
		// A retried request after a timeout must not start the work twice.
		second, err := api.CreateRun(req)
		if err != nil {
			t.Fatalf("second create: %v", err)
		}
		if second.ID != first.ID {
			t.Errorf("a repeated idempotency key created a second run (%s and %s)", first.ID, second.ID)
		}

		runs, err := api.ListRuns()
		if err != nil {
			t.Fatalf("list runs: %v", err)
		}
		matches := 0
		for _, r := range runs {
			if r.ID == first.ID {
				matches++
			}
		}
		if matches != 1 {
			t.Errorf("the run appears %d times in the list, want once", matches)
		}
	})

	t.Run("Fail - no key, a nonsense key, and a revoked key are all refused", func(t *testing.T) {
		anonymous := stack.Client()
		if _, err := anonymous.CreateRun(harness.CreateRunRequest{Queue: "build", Type: "noop"}); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Errorf("create a run with no key: err = %v, want 401", err)
		}

		bogus := stack.Client().WithAPIKey("lute_sk_not_a_real_key")
		if _, err := bogus.CreateRun(harness.CreateRunRequest{Queue: "build", Type: "noop"}); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Errorf("create a run with a forged key: err = %v, want 401", err)
		}

		doomed, err := admin.CreateAPIKey("to-be-revoked")
		if err != nil {
			t.Fatalf("create key: %v", err)
		}
		revoked := admin.WithAPIKey(doomed.Token)
		if _, err := revoked.ListRuns(); err != nil {
			t.Fatalf("the new key does not work: %v", err)
		}
		if err := admin.RevokeAPIKey(doomed.ID); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		if _, err := revoked.ListRuns(); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Errorf("using a revoked key: err = %v, want 401", err)
		}
	})

	t.Run("Fail - a run id that is not the caller's own is not readable", func(t *testing.T) {
		mine, err := api.CreateRun(harness.CreateRunRequest{Queue: "build", Type: "noop"})
		if err != nil {
			t.Fatalf("create run: %v", err)
		}

		// There is no second account yet, so this only checks a foreign run id reads as absent.
		if _, err := api.GetRun("000000000000000000000000"); harness.StatusOf(err) != http.StatusNotFound {
			t.Errorf("get a run id that belongs to nobody: err = %v, want 404", err)
		}
		if _, err := api.GetRun(mine.ID); err != nil {
			t.Errorf("get my own run: %v", err)
		}
	})
}

func TestRunWebhooks(t *testing.T) {
	stack := newStack(t)
	admin := stack.AdminClient()
	stack.ConnectedAgent(admin, "webhook-host", harness.WithQueues("build"))

	key, err := admin.CreateAPIKey("webhooks")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	api := admin.WithAPIKey(key.Token)

	receiver := newWebhookReceiver(t)

	t.Run("Success - a successful run delivers started and completed, signed", func(t *testing.T) {
		run, err := api.CreateRun(harness.CreateRunRequest{
			Queue:   "build",
			Type:    "container",
			Payload: containerPayload(t, "bash:5", "echo webhook build"),
			Webhook: &harness.WebhookSpec{
				URL:    receiver.URL(),
				Events: []string{"run.started", "run.completed"},
			},
		})
		if err != nil {
			t.Fatalf("create run: %v", err)
		}
		if run.WebhookSecret == "" {
			t.Fatal("no webhook secret returned; the receiver cannot verify anything")
		}

		harness.WaitFor(t, 2*time.Minute, "the completed callback to arrive", func() bool {
			return receiver.Has("run.completed")
		})
		if !receiver.Has("run.started") {
			t.Error("no run.started callback arrived")
		}

		delivery := receiver.Get(t, "run.completed")
		// A receiver must reject a badly signed callback, so it is as good as undelivered.
		if !verifySignature(delivery, run.WebhookSecret) {
			t.Errorf("the callback's signature does not verify with the returned secret\nheaders: %v", delivery.Header)
		}
		if got := delivery.Body["event"]; got != "run.completed" {
			t.Errorf("payload event = %v, want run.completed", got)
		}
	})

	t.Run("Success - a failing run reports failed once it is really dead", func(t *testing.T) {
		receiver.Reset()

		run, err := api.CreateRun(harness.CreateRunRequest{
			Queue:      "build",
			Type:       "container",
			MaxRetries: 2,
			Payload:    containerPayload(t, "bash:5", "exit 9"),
			Webhook: &harness.WebhookSpec{
				URL:    receiver.URL(),
				Events: []string{"run.failed"},
			},
		})
		if err != nil {
			t.Fatalf("create run: %v", err)
		}

		harness.WaitFor(t, 3*time.Minute, "the failed callback to arrive", func() bool {
			return receiver.Has("run.failed")
		})

		// Only the final failure is reported, not each retried attempt.
		if n := receiver.Count("run.failed"); n != 1 {
			t.Errorf("run.failed delivered %d times, want once (per attempt notifications are noise)", n)
		}
		final, err := api.GetRun(run.ID)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		if final.Status != "dead" {
			t.Errorf("run status = %q, want dead", final.Status)
		}
	})
}

type webhookReceiver struct {
	srv *httptest.Server

	mu         sync.Mutex
	deliveries []delivery
}

type delivery struct {
	Header http.Header
	Raw    []byte
	Body   map[string]any
}

func newWebhookReceiver(t *testing.T) *webhookReceiver {
	t.Helper()
	r := &webhookReceiver{}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		raw, _ := io.ReadAll(req.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)

		r.mu.Lock()
		r.deliveries = append(r.deliveries, delivery{Header: req.Header.Clone(), Raw: raw, Body: body})
		r.mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(r.srv.Close)
	return r
}

func (r *webhookReceiver) URL() string { return r.srv.URL + "/hook" }

func (r *webhookReceiver) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deliveries = nil
}

func (r *webhookReceiver) Count(event string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, d := range r.deliveries {
		if d.Header.Get("X-Lute-Event") == event {
			n++
		}
	}
	return n
}

func (r *webhookReceiver) Has(event string) bool { return r.Count(event) > 0 }

func (r *webhookReceiver) Get(t *testing.T, event string) delivery {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range r.deliveries {
		if d.Header.Get("X-Lute-Event") == event {
			return d
		}
	}
	t.Fatalf("no %s delivery received", event)
	return delivery{}
}

func verifySignature(d delivery, secret string) bool {
	header := d.Header.Get("X-Lute-Signature")
	var ts, sig string
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			ts = value
		case "v1":
			sig = value
		}
	}
	if ts == "" || sig == "" {
		return false
	}
	if _, err := strconv.ParseInt(ts, 10, 64); err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%s.", ts)
	mac.Write(d.Raw)
	return hmac.Equal([]byte(sig), []byte(hex.EncodeToString(mac.Sum(nil))))
}
