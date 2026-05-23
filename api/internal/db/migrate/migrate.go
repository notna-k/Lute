package migrate

import (
	"gorm.io/gorm"

	"github.com/lute/api/internal/db/models"
)

// RegisteredModels lists all migrated tables in dependency-safe order (parents first).
func RegisteredModels() []any {
	return []any{
		&models.User{},
		&models.RefreshToken{},
		&models.Worker{},
		&models.Command{},
		&models.UptimeSnapshot{},
		&models.WorkerSnapshot{},
		&models.JobExecution{},
		&models.APIKey{},
		&models.Run{},
		&models.WebhookDelivery{},
		&models.QueueSlot{},
		&models.QueueDLQ{},
		&models.QueueStatsMinute{},
	}
}

// Run applies AutoMigrate plus secondary indexes neither dialect expresses well in struct tags alone.
func Run(db *gorm.DB) error {
	for _, model := range RegisteredModels() {
		if err := db.AutoMigrate(model); err != nil {
			return err
		}
	}
	return ApplySecondaryIndexes(db)
}
