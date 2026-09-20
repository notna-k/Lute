package jobdefs

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lute/api/internal/db/models"
)

// KnownTypes is the set of parameter types the schema supports. A definition
// naming anything else is rejected at sync time rather than silently accepting
// arbitrary values at trigger time.
var KnownTypes = map[string]bool{
	"string": true, "number": true, "bool": true, "select": true,
	"multiselect": true, "date": true, "datetime": true, "secret": true,
}

// Resolved is the outcome of validating a trigger payload against a schema.
type Resolved struct {
	// Env maps each parameter's EnvVar to its stringified value, ready to hand
	// to the container as request_params.
	Env map[string]string
	// Environment is the value of a parameter literally named "environment",
	// surfaced on the build list. Empty when the job has no such parameter.
	Environment string
}

// ValidationError aggregates per-field problems for a 400 response.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for k, v := range e.Fields {
		parts = append(parts, fmt.Sprintf("%s: %s", k, v))
	}
	return "invalid parameters — " + strings.Join(parts, "; ")
}

// Validate checks submitted values against the job's parameter schema and
// resolves them into container env vars. Secret parameters are not accepted
// inline; they are resolved worker-side from their secretRef (not implemented
// yet) and therefore skipped here.
func Validate(fields []models.ParameterField, values map[string]any) (*Resolved, error) {
	res := &Resolved{Env: map[string]string{}}
	verr := &ValidationError{Fields: map[string]string{}}

	for _, f := range fields {
		if f.Type == "secret" {
			continue
		}

		raw, present := values[f.Name]
		if !present || raw == nil || raw == "" {
			if f.Default != nil {
				raw = f.Default
				present = true
			}
		}
		if !present || raw == nil || raw == "" {
			if f.Required {
				verr.Fields[f.Name] = "required"
			}
			continue
		}

		str, err := coerce(f, raw)
		if err != nil {
			verr.Fields[f.Name] = err.Error()
			continue
		}
		res.Env[f.EnvVar] = str
		if f.Name == "environment" {
			res.Environment = str
		}
	}

	if len(verr.Fields) > 0 {
		return nil, verr
	}
	return res, nil
}

func coerce(f models.ParameterField, raw any) (string, error) {
	switch f.Type {
	case "string":
		return fmt.Sprintf("%v", raw), nil

	case "number":
		switch v := raw.(type) {
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64), nil
		case int:
			return strconv.Itoa(v), nil
		case string:
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return "", fmt.Errorf("must be a number")
			}
			return v, nil
		default:
			return "", fmt.Errorf("must be a number")
		}

	case "bool":
		switch v := raw.(type) {
		case bool:
			return strconv.FormatBool(v), nil
		case string:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return "", fmt.Errorf("must be true or false")
			}
			return strconv.FormatBool(b), nil
		default:
			return "", fmt.Errorf("must be true or false")
		}

	case "select":
		s := fmt.Sprintf("%v", raw)
		if !hasOption(f.Options, s) {
			return "", fmt.Errorf("must be one of the allowed options")
		}
		return s, nil

	case "multiselect":
		items, ok := toStringSlice(raw)
		if !ok {
			return "", fmt.Errorf("must be a list")
		}
		for _, it := range items {
			if !hasOption(f.Options, it) {
				return "", fmt.Errorf("%q is not an allowed option", it)
			}
		}
		return strings.Join(items, ","), nil

	case "date":
		s := fmt.Sprintf("%v", raw)
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return "", fmt.Errorf("must be a YYYY-MM-DD date")
		}
		return s, nil

	case "datetime":
		s := fmt.Sprintf("%v", raw)
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			// Allow a plain date too.
			if _, err2 := time.Parse("2006-01-02", s); err2 != nil {
				return "", fmt.Errorf("must be an ISO-8601 datetime")
			}
		}
		return s, nil

	default:
		// Unreachable for synced definitions — Sync rejects unknown types.
		return fmt.Sprintf("%v", raw), nil
	}
}

func hasOption(opts []models.ParameterOption, value string) bool {
	for _, o := range opts {
		if o.Value == value {
			return true
		}
	}
	return false
}

func toStringSlice(raw any) ([]string, bool) {
	switch v := raw.(type) {
	case []string:
		return v, true
	case []any:
		out := make([]string, 0, len(v))
		for _, it := range v {
			out = append(out, fmt.Sprintf("%v", it))
		}
		return out, true
	default:
		return nil, false
	}
}
