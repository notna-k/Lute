package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UptimeSnapshot is a per-user snapshot of machine counts at a point in time (for dashboard uptime graph).
type UptimeSnapshot struct {
	UserID primitive.ObjectID `json:"user_id" bson:"user_id"`
	At     time.Time          `json:"at" bson:"at"`
	Alive  int                `json:"alive" bson:"alive"`
	Dead   int                `json:"dead" bson:"dead"`
	Total  int                `json:"total" bson:"total"`
}
