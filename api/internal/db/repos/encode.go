package repos

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lute/api/internal/db/id"
)

func scanID(ns sql.NullString) id.ID {
	if !ns.Valid {
		return id.ID("")
	}
	return id.ID(ns.String)
}

func idArg(i id.ID) any {
	if i.IsZero() {
		return nil
	}
	return i.Hex()
}

func timeMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func readTime(ms sql.NullInt64) time.Time {
	if !ms.Valid || ms.Int64 == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms.Int64).UTC()
}

func readTimePtr(ms sql.NullInt64) *time.Time {
	if !ms.Valid || ms.Int64 == 0 {
		return nil
	}
	t := time.UnixMilli(ms.Int64).UTC()
	return &t
}

func timePtrArg(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UnixMilli()
}

func marshalStringMap(m map[string]string) (sql.NullString, error) {
	if m == nil {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func unmarshalStringMap(ns sql.NullString) (map[string]string, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(ns.String), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func marshalIfaceMap(m map[string]interface{}) (sql.NullString, error) {
	if m == nil {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func unmarshalIfaceMap(ns sql.NullString) (map[string]interface{}, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(ns.String), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func marshalStringSlice(s []string) (sql.NullString, error) {
	if s == nil {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func unmarshalStringSlice(ns sql.NullString) ([]string, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil
	}
	var s []string
	if err := json.Unmarshal([]byte(ns.String), &s); err != nil {
		return nil, err
	}
	return s, nil
}
