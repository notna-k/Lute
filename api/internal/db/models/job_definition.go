package models

import "encoding/json"

// ParameterOption is a selectable choice for select/multiselect parameters.
type ParameterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
	Tone  string `json:"tone,omitempty"`
}

// ParameterField describes one typed input a job accepts. The schema both
// renders the trigger UI and validates the trigger payload server-side.
//
// Type is one of: string, number, bool, select, multiselect, date, datetime, secret.
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

// JobSpec is everything a definition's YAML file says about the job — the part
// that can be edited in the panel and compared against Git.
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

// Equal compares two specs by content. It goes through JSON so that nil and
// empty collections, and a YAML uint64 default against a JSON float64 one,
// compare as the same value.
func (s JobSpec) Equal(o JobSpec) bool {
	a, errA := json.Marshal(s)
	b, errB := json.Marshal(o)
	return errA == nil && errB == nil && string(a) == string(b)
}

// JobDefinition is a reusable job. Git is the source of truth: the sync writes
// each YAML file's spec here, and remembers it in GitSpec. The panel may edit
// the live spec, which then drifts from GitSpec until the next change in Git
// overwrites it (or someone commits the panel's version).
type JobDefinition struct {
	BaseModel
	Slug    string `json:"slug" gorm:"column:slug;uniqueIndex;size:191;not null"`
	JobSpec `gorm:"embedded"`
	// SourcePath and SourceCommit locate the YAML file this came from. A
	// definition created in the panel has neither; one whose file was deleted
	// keeps its last path, which is how the panel tells the two apart.
	SourcePath   string `json:"sourcePath"`
	SourceCommit string `json:"sourceCommit"`
	// GitSpec is the spec as Git last stated it, or nil when no file in Git
	// defines this slug (created in the panel, or removed from Git).
	GitSpec *JobSpec `json:"gitSpec,omitempty" gorm:"serializer:json"`
}

// Git states of a definition, derived from its live spec against GitSpec.
const (
	GitSynced   = "synced"   // matches its YAML file
	GitModified = "modified" // edited in the panel since the last Git change
	GitManual   = "manual"   // created in the panel, never in Git
	GitRemoved  = "removed"  // its YAML file was deleted; kept because prune is off
)

// GitState reports how this definition relates to Git.
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
