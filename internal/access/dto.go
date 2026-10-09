package access

import "github.com/google/uuid"

type Credentials struct {
	Email    string
	Password string
}

type PasswordChange struct {
	CurrentPassword *string
	NewPassword     string
}

type CreateUser struct {
	Email        string
	DisplayName  string
	Password     string
	IsSuperadmin bool
	IsService    bool
}

type UpdateUser struct {
	DisplayName  *string
	Status       *string
	IsSuperadmin *bool
	IsService    *bool
}

type GroupName struct {
	Name string
}

type PageQuery struct {
	Limit  *int64
	Offset *int64
}

type PageRequest struct {
	Limit  uint32
	Offset uint64
}

type Page[T any] struct {
	Items  []T
	Total  uint64
	Limit  uint32
	Offset uint64
}

type NewUser struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	PasswordHash string
	IsSuperadmin bool
	IsService    bool
}

type UserChanges struct {
	DisplayName  *string
	Status       *UserStatus
	IsSuperadmin *bool
	IsService    *bool
}

type NewSession struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	TokenHash           [32]byte
	AbsoluteTimeoutSecs uint64
	Method              string
}

type NewGroup struct {
	ID   uuid.UUID
	Name string
}

type CreateToken struct {
	Name          string
	Scopes        []string
	ExpiresInDays *int64
}

type NewToken struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Name          string
	Prefix        string
	Hash          [32]byte
	Scopes        Scopes
	ExpiresInDays *uint32
}

type TokenQuery struct {
	Prefix *string
	Page   PageQuery
}

type TokenFilter struct {
	Prefix string
	Page   PageRequest
}
