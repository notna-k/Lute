package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// MilliTime is stored as Unix milliseconds (BIGINT) and serialised to JSON as RFC3339.
type MilliTime struct {
	time.Time
}

func NewMilliTime(t time.Time) MilliTime {
	if t.IsZero() {
		return MilliTime{}
	}
	return MilliTime{Time: t.UTC()}
}

func (m MilliTime) Value() (driver.Value, error) {
	if m.IsZero() {
		return nil, nil
	}
	return m.UTC().UnixMilli(), nil
}

func (m *MilliTime) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		m.Time = time.Time{}
	case int64:
		m.Time = time.UnixMilli(v).UTC()
	default:
		return fmt.Errorf("cannot scan MilliTime from %T", value)
	}
	return nil
}

func (m MilliTime) MarshalJSON() ([]byte, error) {
	if m.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(m.UTC().Format(time.RFC3339Nano))
}

func (m *MilliTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		m.Time = time.Time{}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return err
	}
	m.Time = t.UTC()
	return nil
}

// GormDataType must stay parameterless to satisfy GORM's interface; otherwise GORM
// falls back to the embedded time.Time and creates a timestamptz column.
func (MilliTime) GormDataType() string {
	return "bigint"
}
