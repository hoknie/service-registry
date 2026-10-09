package access

import "github.com/google/uuid"

type Identity struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Provider    string
	Subject     string
	Email       *string
	CreatedAt   string
	LastLoginAt string
}

type NewIdentity struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	Provider string
	Subject  string
	Email    *string
}

type NewLoginState struct {
	ID          uuid.UUID
	BrowserHash [32]byte
	Provider    string
	State       string
	Nonce       string
	Verifier    string
	Next        *string
	LinkUserID  *uuid.UUID
	TTLSecs     int
}

type LoginState struct {
	Provider   string
	Nonce      string
	Verifier   string
	Next       *string
	LinkUserID *uuid.UUID
}
