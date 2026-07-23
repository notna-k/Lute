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

func (m MilliTime) Value() (driver.Value, error) {
	if m.Time.IsZero() {
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
	case time.Time:
		m.Time = v.UTC()
	case string:
		n, err := parseMilliOrTime(v)
		if err != nil {
			return err
		}
		m.Time = n
	case []byte:
		n, err := parseMilliOrTime(string(v))
		if err != nil {
			return err
		}
		m.Time = n
	default:
		return fmt.Errorf("cannot scan MilliTime from %T", value)
	}
	return nil
}

func (m MilliTime) MarshalJSON() ([]byte, error) {
	if m.Time.IsZero() {
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

// GormDataType makes GORM store MilliTime as an integer column (BIGINT) on every
// dialect. The signature MUST be parameterless to satisfy GORM's
// GormDataTypeInterface — with a parameter GORM ignores it and falls back to the
// embedded time.Time, producing a timestamptz column that rejects our millis on
// PostgreSQL (SQLite's loose typing hid this).
func (MilliTime) GormDataType() string {
	return "bigint"
}

// parseMilliOrTime accepts either a numeric unix-millis string or an RFC3339 /
// SQLite datetime string and returns a UTC time.Time. Lets us tolerate legacy
// rows written before MilliTime stored as bigint.
func parseMilliOrTime(s string) (time.Time, error) {
	var n int64
	if _, err := fmt.Sscan(s, &n); err == nil {
		return time.UnixMilli(n).UTC(), nil
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse MilliTime from %q", s)
}

func (m MilliTime) IsZero() bool {
	return m.Time.IsZero()
}
