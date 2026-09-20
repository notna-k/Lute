package jobdefs

import (
	"encoding/json"

	"github.com/lute/api/internal/db/models"
)

// schemaDiffers reports whether a schema submitted by the panel differs from
// the definition synced from Git.
//
// Comparison is on the canonical JSON projection of the fields, so it tracks
// whatever ParameterField carries without this function needing to know the
// shape. Field order is significant: reordering inputs changes the rendered
// form, so it counts as an edit.
func schemaDiffers(gitSchema, submitted []models.ParameterField) bool {
	if len(gitSchema) != len(submitted) {
		return true
	}
	a, err1 := json.Marshal(gitSchema)
	b, err2 := json.Marshal(submitted)
	if err1 != nil || err2 != nil {
		// Unmarshalable input is treated as drifted: fail toward the stricter
		// path rather than silently accepting a schema we cannot compare.
		return true
	}
	return string(a) != string(b)
}
