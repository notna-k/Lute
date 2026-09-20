package repos

import (
	"context"
	"errors"
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lute/api/internal/db/models"
)

type SettingRepository struct {
	g *gorm.DB
}

func NewSettingRepository(db *gorm.DB) *SettingRepository {
	return &SettingRepository{g: db}
}

func (r *SettingRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

// Get returns the stored value, or the registered default when the key has
// never been written. An unknown key yields "".
func (r *SettingRepository) Get(ctx context.Context, key string) (string, error) {
	var s models.Setting
	err := r.q(ctx).Where("key = ?", key).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.SettingDefaults[key], nil
	}
	if err != nil {
		return "", mapErr(err)
	}
	return s.Value, nil
}

// GetBool reads a boolean setting, falling back to the default on any
// unparseable value so a bad row can never wedge the panel.
func (r *SettingRepository) GetBool(ctx context.Context, key string) (bool, error) {
	raw, err := r.Get(ctx, key)
	if err != nil {
		return false, err
	}
	v, perr := strconv.ParseBool(raw)
	if perr != nil {
		v, _ = strconv.ParseBool(models.SettingDefaults[key])
	}
	return v, nil
}

// Set upserts a key. Callers validate the value before writing.
func (r *SettingRepository) Set(ctx context.Context, key, value string) error {
	s := &models.Setting{Key: key, Value: value}
	return mapErr(r.q(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(s).Error)
}

// All returns every known setting, with defaults filled in for unwritten keys.
func (r *SettingRepository) All(ctx context.Context) (map[string]string, error) {
	var rows []models.Setting
	if err := r.q(ctx).Find(&rows).Error; err != nil {
		return nil, mapErr(err)
	}
	out := make(map[string]string, len(models.SettingDefaults))
	for k, v := range models.SettingDefaults {
		out[k] = v
	}
	for i := range rows {
		if _, known := models.SettingDefaults[rows[i].Key]; known {
			out[rows[i].Key] = rows[i].Value
		}
	}
	return out, nil
}
