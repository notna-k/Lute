package types

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// MilliTime stores wall-clock instants as Unix milliseconds in SQL (BIGINT/INTEGER)
// and as RFC3339 strings in JSON for API compatibility.
type MilliTime struct {
	time.Time
}

func NewMilliTime(t time.Time) MilliTime {
	if t.IsZero() {
		return MilliTime{}
	}
	return MilliTime{Time: t.UTC()}
}

func (m *MilliTime) Value() (driver.Value, error) {
	if m == nil || m.Time.IsZero() {
		return nil, nil
	}
	return m.UTC().UnixMilli(), nil
}

func (m *MilliTime) Scan(value interface{}) error {
	if m == nil {
		return fmt.Errorf("MilliTime.Scan on nil receiver")
	}
	if value == nil {
		m.Time = time.Time{}
		return nil
	}
	switch v := value.(type) {
	case int64:
		m.Time = time.UnixMilli(v).UTC()
	case int:
		m.Time = time.UnixMilli(int64(v)).UTC()
	case []byte:
		var n int64
		if _, err := fmt.Sscan(string(v), &n); err != nil {
			return fmt.Errorf("scan MilliTime from []byte: %w", err)
		}
		m.Time = time.UnixMilli(n).UTC()
	default:
		return fmt.Errorf("cannot scan MilliTime from %T", value)
	}
	return nil
}

func (m *MilliTime) MarshalJSON() ([]byte, error) {
	if m == nil || m.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(m.UTC().Format(time.RFC3339Nano))
}

func (m *MilliTime) UnmarshalJSON(data []byte) error {
	if m == nil {
		return fmt.Errorf("MilliTime.UnmarshalJSON on nil receiver")
	}
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
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return err
		}
	}
	m.Time = t.UTC()
	return nil
}

func (m MilliTime) GormDataType(_ string) string {
	return "bigint"
}

func (m MilliTime) IsZero() bool {
	return m.Time.IsZero()
}
