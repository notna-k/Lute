package models

// User represents a human account in Lute (Firebase linkage).
type User struct {
	BaseModel
	Email       string `json:"email" gorm:"index:idx_users_email"`
	DisplayName string `json:"display_name"`
	FirebaseUID string `json:"firebase_uid" gorm:"uniqueIndex:idx_users_firebase_uid"`
}

func (*User) TableName() string { return "users" }
