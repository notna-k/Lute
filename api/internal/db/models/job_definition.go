package models

import (
	"encoding/json"
	"strings"
)

type ParameterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
	Tone  string `json:"tone,omitempty"`
}

// ParameterField is one typed job input; the schema renders the trigger form and validates it.
type ParameterField struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Label       string            `json:"label"`
	EnvVar      string            `json:"envVar"`
	Description string            `json:"description,omitempty"`
	Required    bool              `json:"required,omitempty"`
	Default     any               `json:"default,omitempty"`
	Options     []ParameterOption `json:"options,omitempty"`
	SecretRef   string            `json:"secretRef,omitempty"`
}

// EnvName is the container env var for this parameter. Without envVar it derives one
// from the name, because the runtime rejects an unnamed variable.
func (f ParameterField) EnvName() string {
	if f.EnvVar != "" {
		return f.EnvVar
	}
	return deriveEnvName(f.Name)
}

// deriveEnvName upper-cases name and maps non-alphanumerics to "_"; "" if nothing usable remains.
func deriveEnvName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - ('a' - 'A'))
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return ""
	}
	if out[0] >= '0' && out[0] <= '9' {
		// A variable name cannot start with a digit.
		return "P_" + out
	}
	return out
}

// JobSpec is what a definition's YAML says: the part the panel edits and compares against Git.
type JobSpec struct {
	Name          string            `json:"name"`
	Description   string            `json:"description,omitempty"`
	Queue         string            `json:"queue"`
	LabelSelector map[string]string `json:"labelSelector,omitempty" gorm:"serializer:json"`
	Runtime       string            `json:"runtime"`
	Command       string            `json:"command"`
	SourceRepo    string            `json:"sourceRepo,omitempty"`
	Parameters    []ParameterField  `json:"parameters,omitempty" gorm:"serializer:json"`
}

// Equal compares via JSON, so nil and empty collections, and a YAML uint64 against a
// JSON float64, count as equal.
func (s JobSpec) Equal(o JobSpec) bool {
	a, errA := json.Marshal(s)
	b, errB := json.Marshal(o)
	return errA == nil && errB == nil && string(a) == string(b)
}

// JobDefinition is a reusable job. The sync writes each file's spec here and into
// GitSpec; a panel edit drifts from GitSpec until the file next changes.
type JobDefinition struct {
	BaseModel
	Slug    string `json:"slug" gorm:"column:slug;uniqueIndex;size:191;not null"`
	JobSpec `gorm:"embedded"`
	// SourcePath is empty for a panel-created definition; one whose file was deleted keeps it.
	SourcePath   string `json:"sourcePath"`
	SourceCommit string `json:"sourceCommit"`
	// GitSpec is nil when no file in Git defines this slug.
	GitSpec *JobSpec `json:"gitSpec,omitempty" gorm:"serializer:json"`
}

const (
	GitSynced   = "synced"   // matches its YAML file
	GitModified = "modified" // edited in the panel since the last Git change
	GitManual   = "manual"   // created in the panel, never in Git
	GitRemoved  = "removed"  // its YAML file was deleted; kept because prune is off
)

func (d *JobDefinition) GitState() string {
	switch {
	case d.GitSpec == nil && d.SourcePath != "":
		return GitRemoved
	case d.GitSpec == nil:
		return GitManual
	case !d.Equal(*d.GitSpec):
		return GitModified
	default:
		return GitSynced
	}
}

func (*JobDefinition) TableName() string { return "job_definitions" }
