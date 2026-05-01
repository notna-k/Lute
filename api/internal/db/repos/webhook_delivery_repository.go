package repos

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/models"
)

type WebhookDeliveryRepository struct {
	*Repository
}

func NewWebhookDeliveryRepository(db *mongo.Database) *WebhookDeliveryRepository {
	return &WebhookDeliveryRepository{Repository: NewRepository(db, connection.CollectionWebhookDeliveries)}
}

func (r *WebhookDeliveryRepository) Create(ctx context.Context, d *models.WebhookDelivery) error {
	d.BeforeCreate()
	_, err := r.Collection.InsertOne(ctx, d)
	return err
}

// ClaimDue atomically selects up to `limit` pending deliveries whose retry time has come,
// flipping them to "in_flight" so concurrent dispatchers do not pick the same row.
func (r *WebhookDeliveryRepository) ClaimDue(ctx context.Context, limit int64) ([]models.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 20
	}
	now := time.Now()

	filter := bson.M{
		"status":        "pending",
		"next_retry_at": bson.M{"$lte": now},
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "next_retry_at", Value: 1}}).
		SetLimit(limit)

	cur, err := r.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cur.Close(ctx) }()

	var candidates []models.WebhookDelivery
	if err := cur.All(ctx, &candidates); err != nil {
		return nil, err
	}

	claimed := make([]models.WebhookDelivery, 0, len(candidates))
	for _, d := range candidates {
		res, err := r.Collection.UpdateOne(
			ctx,
			bson.M{"_id": d.ID, "status": "pending"},
			bson.M{"$set": bson.M{"status": "in_flight", "updated_at": now}},
		)
		if err != nil || res.MatchedCount == 0 {
			continue
		}
		d.Status = "in_flight"
		claimed = append(claimed, d)
	}
	return claimed, nil
}

// MarkDelivered records a successful webhook delivery.
func (r *WebhookDeliveryRepository) MarkDelivered(ctx context.Context, id primitive.ObjectID, attempts, status int) error {
	now := time.Now()
	_, err := r.Collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"status":          "delivered",
			"attempts":        attempts,
			"response_status": status,
			"delivered_at":    now,
			"updated_at":      now,
		}},
	)
	return err
}

// MarkRetry reschedules a delivery with exponential backoff up to MaxAttempts,
// or marks it "failed" when attempts are exhausted.
func (r *WebhookDeliveryRepository) MarkRetry(
	ctx context.Context,
	id primitive.ObjectID,
	attempts int,
	maxAttempts int,
	lastErr string,
	responseStatus int,
) error {
	now := time.Now()
	if attempts >= maxAttempts {
		_, err := r.Collection.UpdateOne(
			ctx,
			bson.M{"_id": id},
			bson.M{"$set": bson.M{
				"status":          "failed",
				"attempts":        attempts,
				"response_status": responseStatus,
				"last_error":      lastErr,
				"updated_at":      now,
			}},
		)
		return err
	}
	delay := time.Duration(1<<uint(attempts)) * 30 * time.Second
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	_, err := r.Collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{
			"status":          "pending",
			"attempts":        attempts,
			"response_status": responseStatus,
			"last_error":      lastErr,
			"next_retry_at":   now.Add(delay),
			"updated_at":      now,
		}},
	)
	return err
}

// ListByJobID returns deliveries tied to a job (for debugging / run details).
func (r *WebhookDeliveryRepository) ListByJobID(ctx context.Context, jobID string) ([]models.WebhookDelivery, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := r.Collection.Find(ctx, bson.M{"job_id": jobID}, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []models.WebhookDelivery
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []models.WebhookDelivery{}
	}
	return out, nil
}
