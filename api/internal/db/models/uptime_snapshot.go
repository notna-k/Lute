package models

import (
	"time"

	"github.com/lute/api/internal/db/id"
)

// UptimeSnapshot is a per-user snapshot of machine counts at a point in time (for dashboard uptime graph).
type UptimeSnapshot struct {
	UserID id.ID     `json:"user_id"`
	At     time.Time `json:"at"`
	Alive  int       `json:"alive"`
	Dead   int       `json:"dead"`
	Total  int       `json:"total"`
}
