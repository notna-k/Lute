package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"github.com/lute/api/config"
)

// Collection names used by the app (must match repository.NewRepository)
const (
	CollectionMachines        = "machines"
	CollectionUsers           = "users"
	CollectionCommands        = "commands"
	CollectionUptimeSnapshots = "uptime_snapshots"
	CollectionMachineSnapshots = "machine_snapshots"
)

type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func NewMongoDB(cfg *config.Config) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.MongoDB.ConnectTimeout)
	defer cancel()

	clientOptions := options.Client().
		ApplyURI(cfg.MongoDB.URI).
		SetMaxPoolSize(cfg.MongoDB.MaxPoolSize).
		SetReadPreference(readpref.Primary())

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping to verify connection
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := client.Database(cfg.MongoDB.Database)

	m := &MongoDB{Client: client, Database: db}
	if err := m.EnsureCollections(ctx); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("failed to ensure collections: %w", err)
	}

	return m, nil
}

// EnsureCollections creates the required collections if they don't exist,
// so the "lute" database and collections appear as soon as the API starts.
func (m *MongoDB) EnsureCollections(ctx context.Context) error {
	for _, name := range []string{CollectionMachines, CollectionUsers, CollectionCommands, CollectionUptimeSnapshots, CollectionMachineSnapshots} {
		if err := m.Database.CreateCollection(ctx, name); err != nil {
			// Code 48 = namespace already exists
			var ce mongo.CommandError
			if errors.As(err, &ce) && ce.HasErrorCode(48) {
				continue
			}
			return err
		}
		log.Printf("MongoDB: created collection %s", name)
	}
	// TTL index on uptime_snapshots.at: expire documents after 30 days
	coll := m.Database.Collection(CollectionUptimeSnapshots)
	ttlSeconds := int32(30 * 24 * 3600)
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.M{"at": 1},
		Options: options.Index().SetExpireAfterSeconds(ttlSeconds),
	})
	if err != nil {
		var ce mongo.CommandError
		if errors.As(err, &ce) && (ce.HasErrorCode(85) || ce.HasErrorCode(86)) {
			// Index with same key or name already exists
			return nil
		}
		return fmt.Errorf("create uptime_snapshots TTL index: %w", err)
	}
	log.Printf("MongoDB: created TTL index on %s.at", CollectionUptimeSnapshots)
	// TTL index on machine_snapshots.at: expire after 30 days
	machineSnapColl := m.Database.Collection(CollectionMachineSnapshots)
	_, err = machineSnapColl.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.M{"at": 1},
		Options: options.Index().SetExpireAfterSeconds(ttlSeconds),
	})
	if err != nil {
		var ce mongo.CommandError
		if errors.As(err, &ce) && (ce.HasErrorCode(85) || ce.HasErrorCode(86)) {
			return nil
		}
		return fmt.Errorf("create machine_snapshots TTL index: %w", err)
	}
	log.Printf("MongoDB: created TTL index on %s.at", CollectionMachineSnapshots)
	// Compound index for GetByMachineIDs (machine_id + at) to avoid whole collection scan
	_, err = machineSnapColl.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "machine_id", Value: 1}, {Key: "at", Value: 1}},
	})
	if err != nil {
		var ce mongo.CommandError
		if errors.As(err, &ce) && (ce.HasErrorCode(85) || ce.HasErrorCode(86)) {
			// index already exists
		} else {
			return fmt.Errorf("create machine_snapshots compound index: %w", err)
		}
	}
	// Indexes on machines for List/GetByUserID (user_id) and GetPublic (is_public)
	machinesColl := m.Database.Collection(CollectionMachines)
	for _, idx := range []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}}},
		{Keys: bson.D{{Key: "is_public", Value: 1}}},
	} {
		_, err = machinesColl.Indexes().CreateOne(ctx, idx)
		if err != nil {
			var ce mongo.CommandError
			if errors.As(err, &ce) && (ce.HasErrorCode(85) || ce.HasErrorCode(86)) {
				continue
			}
			return fmt.Errorf("create machines index: %w", err)
		}
	}
	return nil
}

func (m *MongoDB) Close(ctx context.Context) error {
	return m.Client.Disconnect(ctx)
}

func (m *MongoDB) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return m.Client.Ping(ctx, readpref.Primary())
}
