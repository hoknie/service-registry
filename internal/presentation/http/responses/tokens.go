package responses

import (
	"github.com/google/uuid"

	accessservices "svc-registry/internal/feature/access/services"
	accesstypes "svc-registry/internal/feature/access/types"
)

type Token struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	Status     string     `json:"status"`
	CreatedAt  string     `json:"created_at"`
	ExpiresAt  *string    `json:"expires_at"`
	LastUsedAt *string    `json:"last_used_at"`
	RevokedAt  *string    `json:"revoked_at"`
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	UserEmail  *string    `json:"user_email,omitempty"`
	Secret     *string    `json:"secret,omitempty"`
}

func TokenOf(t accesstypes.Token) Token {
	return Token{
		ID: t.ID, Name: t.Name, Prefix: t.Prefix, Scopes: t.Scopes.Strings(), Status: string(t.Status),
		CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt, LastUsedAt: t.LastUsedAt, RevokedAt: t.RevokedAt,
	}
}

func OwnedTokenOf(t accesstypes.Token) Token {
	out := TokenOf(t)
	id, email := t.UserID, t.UserEmail
	out.UserID, out.UserEmail = &id, &email
	return out
}

func IssuedTokenOf(i accessservices.IssuedToken) Token {
	out := TokenOf(i.Token)
	out.Secret = new(i.Secret)
	return out
}
