package models

// User represents a user in the system
type User struct {
	BaseModel
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	FirebaseUID string `json:"firebase_uid"`
}
