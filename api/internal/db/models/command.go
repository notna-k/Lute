package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Command represents a queued command for an agent to execute
type Command struct {
	BaseModel `bson:",inline"`
	MachineID primitive.ObjectID `json:"machine_id" bson:"machine_id"`
	Command   string             `json:"command" bson:"command"`
	Args      []string           `json:"args,omitempty" bson:"args,omitempty"`
	Env       map[string]string  `json:"env,omitempty" bson:"env,omitempty"`
	Status    string             `json:"status" bson:"status"` // "pending", "running", "completed", "failed"
	Output    string             `json:"output,omitempty" bson:"output,omitempty"`
	ExitCode  int                `json:"exit_code" bson:"exit_code"`
	Error     string             `json:"error,omitempty" bson:"error,omitempty"`
}
