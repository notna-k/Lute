package models

import (
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/types"
)

// UptimeSnapshot is persisted time-series rollup for dashboard uptime.
type UptimeSnapshot struct {
	ID     uint64 `json:"-" gorm:"primaryKey;autoIncrement"`
	UserID id.ID  `json:"user_id" gorm:"column:user_id;size:24"`
	At     types.MilliTime `json:"at" gorm:"column:at"`
	Alive  int    `json:"alive"`
	Dead   int    `json:"dead"`
	Total  int    `json:"total"`
}

func (*UptimeSnapshot) TableName() string { return "uptime_snapshots" }
