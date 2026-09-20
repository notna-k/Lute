package models

// User represents a human account in Lute.
type User struct {
	BaseModel
	Email        string `json:"email" gorm:"index:idx_users_email;default:''"`
	DisplayName  string `json:"display_name"`
	PasswordHash string `json:"-" gorm:"column:password_hash;default:''"`
}

func (*User) TableName() string { return "users" }
