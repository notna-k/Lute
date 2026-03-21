package models

// User represents a user in the system
type User struct {
	BaseModel   `bson:",inline"`
	Email       string `json:"email" bson:"email"`
	DisplayName string `json:"display_name" bson:"display_name"`
	FirebaseUID string `json:"firebase_uid" bson:"firebase_uid"`
}
