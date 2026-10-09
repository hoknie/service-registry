package access

import "github.com/google/uuid"

type SessionUser struct {
	SessionID uuid.UUID
	User      User
	Method    string
}

const MethodPassword = "password"

func OAuthMethod(provider string) string { return "oauth:" + provider }
