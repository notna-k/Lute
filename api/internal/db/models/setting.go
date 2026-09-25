package models

// Setting is a panel-managed operator switch. It must never encode anything a job definition could express.
type Setting struct {
	BaseModel
	Key   string `json:"key" gorm:"column:key;uniqueIndex;size:191;not null"`
	Value string `json:"value" gorm:"column:value"`
}

func (*Setting) TableName() string { return "settings" }

// AllowAdhocBuilds permits builds whose schema differs from Git; off means every build runs a committed definition.
const AllowAdhocBuilds = "allow_adhoc_builds"

// PruneDefinitions makes the sync delete definitions no YAML file defines, panel-created ones included.
const PruneDefinitions = "prune_definitions"

var SettingDefaults = map[string]string{
	AllowAdhocBuilds: "true",
	PruneDefinitions: "false",
}
