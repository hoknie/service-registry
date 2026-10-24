package types

import (
	"slices"

	"github.com/google/uuid"
)

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

type SessionUser struct {
	SessionID uuid.UUID
	User      User
	Method    string
}

const MethodPassword = "password"

func OAuthMethod(provider string) string { return "oauth:" + provider }

func (c UserChanges) Disables() bool { return c.Status != nil && *c.Status == StatusDisabled }

func (c UserChanges) Demotes() bool { return c.IsSuperadmin != nil && !*c.IsSuperadmin }

func (c UserChanges) MakesService() bool { return c.IsService != nil && *c.IsService }

func LastSuperadmin(activeSuperadmins []uuid.UUID, id uuid.UUID) bool {
	return len(activeSuperadmins) == 1 && slices.Contains(activeSuperadmins, id)
}
