package models

// Setting is a panel-managed key/value knob. These are operator policy
// switches, deliberately kept out of the job-definition YAML: PRODUCT.md's
// canonical serialization rule governs *definitions*, and a setting here must
// never encode anything a definition could express.
type Setting struct {
	BaseModel
	Key   string `json:"key" gorm:"column:key;uniqueIndex;size:191;not null"`
	Value string `json:"value" gorm:"column:value"`
}

func (*Setting) TableName() string { return "settings" }

// AllowAdhocBuilds gates running a template whose schema differs from the
// definition synced from Git (edited in the workbench, or built from scratch).
// Default true — turning it off requires every build to come from a committed
// definition.
const AllowAdhocBuilds = "allow_adhoc_builds"

// SettingDefaults are applied when a key has never been written.
var SettingDefaults = map[string]string{
	AllowAdhocBuilds: "true",
}
