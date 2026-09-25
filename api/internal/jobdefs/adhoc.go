package jobdefs

import (
	"encoding/json"

	"github.com/lute/api/internal/db/models"
)

// schemaDiffers compares the JSON form of both schemas, so it tracks whatever
// ParameterField carries. Order matters: reordering inputs changes the form.
func schemaDiffers(gitSchema, submitted []models.ParameterField) bool {
	if len(gitSchema) != len(submitted) {
		return true
	}
	a, err1 := json.Marshal(gitSchema)
	b, err2 := json.Marshal(submitted)
	if err1 != nil || err2 != nil {
		// Fail toward the stricter path.
		return true
	}
	return string(a) != string(b)
}
