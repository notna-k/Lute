// Package webhooks records run events as deliveries and sends them HMAC-signed, retrying with backoff.
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/db/types"
)

const (
	defaultMaxAttempts  = 6
	requestTimeout      = 10 * time.Second
	DefaultPollInterval = 5 * time.Second
	batchSize           = 20
)

type Emitter struct {
	runs       *repos.RunRepository
	deliveries *repos.WebhookDeliveryRepository
}

func NewEmitter(runs *repos.RunRepository, deliveries *repos.WebhookDeliveryRepository) *Emitter {
	return &Emitter{runs: runs, deliveries: deliveries}
}

// Emit records a delivery if the run subscribes to event. Errors are only logged:
// webhooks must never block the job path.
func (e *Emitter) Emit(ctx context.Context, jobID, event string, payload map[string]interface{}) {
	if e == nil || e.runs == nil || e.deliveries == nil {
		return
	}
	run, err := e.runs.GetByJobID(ctx, jobID)
	if err != nil {
		if !errors.Is(err, repos.ErrNotFound) {
			slog.Error("webhooks: look up run", "job_id", jobID, "err", err)
		}
		return
	}
	if run.WebhookURL == "" || !contains(run.WebhookEvents, event) {
		return
	}

	body := map[string]interface{}{
		"event":     event,
		"run_id":    run.JobID,
		"queue":     run.Queue,
		"type":      run.Type,
		"timestamp": time.Now().Unix(),
		"data":      payload,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		slog.Error("webhooks: marshal event", "job_id", jobID, "event", event, "err", err)
		return
	}

	ts := time.Now().Unix()
	sig := sign(run.WebhookSecret, ts, raw)

	d := &models.WebhookDelivery{
		RunID:           run.ID,
		JobID:           run.JobID,
		UserID:          run.UserID,
		Event:           event,
		URL:             run.WebhookURL,
		Payload:         raw,
		Signature:       sig,
		SignedTimestamp: ts,
		Status:          "pending",
		Attempts:        0,
		MaxAttempts:     defaultMaxAttempts,
		NextRetryAt:     types.NewMilliTime(time.Now()),
	}
	if err := e.deliveries.Create(ctx, d); err != nil {
		slog.Error("webhooks: persist delivery", "job_id", jobID, "event", event, "err", err)
	}
}

type Dispatcher struct {
	deliveries   *repos.WebhookDeliveryRepository
	httpClient   *http.Client
	pollInterval time.Duration
}

func NewDispatcher(deliveries *repos.WebhookDeliveryRepository, pollInterval time.Duration) *Dispatcher {
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}
	return &Dispatcher{
		deliveries:   deliveries,
		httpClient:   &http.Client{Timeout: requestTimeout},
		pollInterval: pollInterval,
	}
}

func (d *Dispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.tick(ctx)
		}
	}
}

func (d *Dispatcher) tick(ctx context.Context) {
	claimed, err := d.deliveries.ClaimDue(ctx, batchSize)
	if err != nil {
		slog.Error("webhooks: claim due deliveries", "err", err)
		return
	}
	for i := range claimed {
		d.send(ctx, &claimed[i])
	}
}

func (d *Dispatcher) send(ctx context.Context, del *models.WebhookDelivery) {
	attempt := del.Attempts + 1
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, del.URL, bytes.NewReader(del.Payload))
	if err != nil {
		_ = d.deliveries.MarkRetry(ctx, del.ID, attempt, del.MaxAttempts, err.Error(), 0)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Lute-Webhook/1.0")
	req.Header.Set("X-Lute-Event", del.Event)
	req.Header.Set("X-Lute-Delivery", del.ID.Hex())
	req.Header.Set("X-Lute-Timestamp", fmt.Sprintf("%d", del.SignedTimestamp))
	req.Header.Set("X-Lute-Signature", fmt.Sprintf("t=%d,v1=%s", del.SignedTimestamp, del.Signature))

	resp, err := d.httpClient.Do(req)
	if err != nil {
		_ = d.deliveries.MarkRetry(ctx, del.ID, attempt, del.MaxAttempts, err.Error(), 0)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_ = d.deliveries.MarkDelivered(ctx, del.ID, attempt, resp.StatusCode)
		return
	}
	_ = d.deliveries.MarkRetry(ctx, del.ID, attempt, del.MaxAttempts,
		fmt.Sprintf("unexpected response %d", resp.StatusCode), resp.StatusCode)
}

func sign(secret string, ts int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d.", ts)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
