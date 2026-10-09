package access

import "github.com/google/uuid"

type UserStatus string

const (
	StatusActive   UserStatus = "active"
	StatusDisabled UserStatus = "disabled"
)

func ParseStatus(s string) (UserStatus, bool) {
	switch UserStatus(s) {
	case StatusActive, StatusDisabled:
		return UserStatus(s), true
	}
	return "", false
}

type User struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	Status       UserStatus
	IsSuperadmin bool
	IsService    bool
	HasPassword  bool
	CreatedAt    string
	UpdatedAt    string
}

type UserCredentials struct {
	User         User
	PasswordHash string
}

type UserSummary struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Status      UserStatus
}
