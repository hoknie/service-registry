package types

import (
	"github.com/google/uuid"

	"svc-registry/internal/platform/apperr"
)

type Principal struct {
	UserID       uuid.UUID
	SessionID    uuid.UUID
	TokenID      uuid.UUID
	Scopes       Scopes
	Nodes        []uuid.UUID
	IsSuperadmin bool
	CatalogAdmin bool
	App          bool
	Method       string
}

func (p Principal) ByToken() bool { return p.TokenID != uuid.Nil }

type Need int

const (
	NeedSession Need = iota
	NeedAny
	NeedRead
	NeedWrite
	NeedWriteWhole
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
	case NeedWriteWhole:
		if p.Nodes != nil {
			return apperr.New(apperr.InsufficientScope)
		}
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
