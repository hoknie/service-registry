package types

import (
	"slices"

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
	NeedUsersRead
	NeedUsersWrite
	NeedGroupsRead
	NeedGroupsWrite
	NeedClustersRead
	NeedClustersWrite
	NeedSettingsRead
	NeedSettingsWrite
)

var globalNeeds = map[Need]Scope{
	NeedUsersRead: ScopeUsersRead, NeedUsersWrite: ScopeUsersWrite,
	NeedGroupsRead: ScopeGroupsRead, NeedGroupsWrite: ScopeGroupsWrite,
	NeedClustersRead: ScopeClustersRead, NeedClustersWrite: ScopeClustersWrite,
	NeedSettingsRead: ScopeSettingsRead, NeedSettingsWrite: ScopeSettingsWrite,
}

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
	default:
		global, ok := globalNeeds[need]
		if !ok {
			return apperr.New(apperr.SessionRequired)
		}
		scope = global
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

func RequireCatalogRoot(p Principal, node *uuid.UUID) error {
	if !p.ByToken() {
		return RequireSuperadmin(p)
	}
	if !p.Scopes.Has(ScopeCatalogRoot) {
		return apperr.New(apperr.InsufficientScope)
	}
	if p.Nodes == nil {
		return RequireSuperadmin(p)
	}
	if !p.CatalogAdmin {
		return apperr.New(apperr.Forbidden)
	}
	if node != nil && !slices.Contains(p.Nodes, *node) {
		return apperr.New(apperr.InsufficientScope)
	}
	return nil
}

func (p Principal) AddsRoots() bool {
	return p.ByToken() && !p.App && p.Nodes != nil && p.CatalogAdmin && p.Scopes.Has(ScopeCatalogRoot)
}
