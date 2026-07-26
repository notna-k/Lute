package models

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

// JobDefinition is a reusable, Git-managed job. It is projected from a YAML
// file in the job-definitions source and upserted into Postgres for serving.
type JobDefinition struct {
	BaseModel
	Slug          string            `json:"slug" gorm:"column:slug;uniqueIndex;size:191;not null"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Queue         string            `json:"queue"`
	LabelSelector map[string]string `json:"labelSelector,omitempty" gorm:"serializer:json"`
	Runtime       string            `json:"runtime"`
	Command       string            `json:"command"`
	SourceRepo    string            `json:"sourceRepo"`
	SourcePath    string            `json:"sourcePath"`
	SourceCommit  string            `json:"sourceCommit"`
	Parameters    []ParameterField  `json:"parameters,omitempty" gorm:"serializer:json"`
	// Origin records who owns this definition. Git-synced rows are rewritten and
	// pruned by every sync; panel-created ones are not, or authoring one would
	// vanish on the next restart.
	Origin string `json:"origin" gorm:"column:origin;size:16;index;default:git"`
}

// Definition origins.
const (
	OriginGit   = "git"
	OriginPanel = "panel"
)

func (*JobDefinition) TableName() string { return "job_definitions" }
