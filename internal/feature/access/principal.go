package access

import (
	"github.com/google/uuid"

	"svc-registry/internal/platform/apperr"
)

type Principal struct {
	UserID       uuid.UUID
	SessionID    uuid.UUID
	TokenID      uuid.UUID
	Scopes       Scopes
	IsSuperadmin bool
	Method       string
}

func (p Principal) ByToken() bool { return p.TokenID != uuid.Nil }

type Need int

const (
	NeedSession Need = iota
	NeedAny
	NeedRead
	NeedWrite
	NeedAdmin
)

func RequireScope(p Principal, need Need) error {
	if !p.ByToken() {
		return nil
	}
	var scope Scope
	switch need {
	case NeedAny:
		return nil
	case NeedRead:
		scope = ScopeRead
	case NeedWrite:
		scope = ScopeWrite
	case NeedAdmin:
		scope = ScopeAdmin
	default:
		return apperr.New(apperr.SessionRequired)
	}
	if !p.Scopes.Has(scope) {
		return apperr.New(apperr.InsufficientScope)
	}
	return nil
}

func RequireSuperadmin(p Principal) error {
	if p.IsSuperadmin {
		return nil
	}
	return apperr.New(apperr.Forbidden)
}
