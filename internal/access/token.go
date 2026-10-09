package access

import "github.com/google/uuid"

type Scope string

const (
	ScopeRead  Scope = "read"
	ScopeWrite Scope = "write"
	ScopeAdmin Scope = "admin"
	ScopeMCP   Scope = "mcp"
)

var AllScopes = []Scope{ScopeRead, ScopeWrite, ScopeAdmin, ScopeMCP}

func ParseScope(s string) (Scope, bool) {
	for _, sc := range AllScopes {
		if string(sc) == s {
			return sc, true
		}
	}
	return "", false
}

type Scopes []Scope

func (s Scopes) Has(scope Scope) bool {
	for _, sc := range s {
		if sc == scope {
			return true
		}
	}
	return false
}

func (s Scopes) Strings() []string {
	out := make([]string, len(s))
	for i, sc := range s {
		out[i] = string(sc)
	}
	return out
}

type TokenStatus string

const (
	TokenActive  TokenStatus = "active"
	TokenExpired TokenStatus = "expired"
	TokenRevoked TokenStatus = "revoked"
)

func ParseTokenStatus(s string) (TokenStatus, bool) {
	switch TokenStatus(s) {
	case TokenActive, TokenExpired, TokenRevoked:
		return TokenStatus(s), true
	}
	return "", false
}

type Token struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	UserEmail  string
	Name       string
	Prefix     string
	Scopes     Scopes
	CreatedAt  string
	ExpiresAt  *string
	LastUsedAt *string
	RevokedAt  *string
	Status     TokenStatus
}

type TokenUser struct {
	TokenID uuid.UUID
	Scopes  Scopes
	User    User
}
