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
	Rights       Rights
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
	NeedPublicationWrite
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
	case NeedPublicationWrite:
		if !p.Scopes.Has(ScopeRead) || !p.Scopes.Has(ScopePublication) || p.Nodes != nil {
			return apperr.New(apperr.InsufficientScope)
		}
		return nil
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

func RequireRight(p Principal, section Section, action Action) error {
	if p.IsSuperadmin || p.Rights.Has(section, action) {
		return nil
	}
	return apperr.New(apperr.Forbidden)
}

func RequireMetrics(p Principal) error {
	if p.EffectiveRights().AllowsScope(ScopeSettingsRead) {
		return nil
	}
	return apperr.New(apperr.Forbidden)
}

func (p Principal) EffectiveRights() Rights {
	base := p.Rights
	if p.IsSuperadmin {
		base = FullRights()
	}
	if p.ByToken() {
		return base.ForScopes(p.Scopes)
	}
	return base.Union(nil)
}

func RequireCatalogRoot(p Principal, node *uuid.UUID, action Action) error {
	if !p.ByToken() {
		return RequireRight(p, SectionCatalog, action)
	}
	if !p.Scopes.Has(ScopeCatalogRoot) {
		return apperr.New(apperr.InsufficientScope)
	}
	if p.Nodes == nil {
		return RequireRight(p, SectionCatalog, action)
	}
	if !p.CatalogAdmin && !p.Rights.Has(SectionCatalog, action) {
		return apperr.New(apperr.Forbidden)
	}
	if node != nil && !slices.Contains(p.Nodes, *node) {
		return apperr.New(apperr.InsufficientScope)
	}
	return nil
}

func (p Principal) AddsRoots() bool {
	return p.ByToken() && !p.App && p.Nodes != nil && p.Scopes.Has(ScopeCatalogRoot) &&
		(p.CatalogAdmin || p.Rights.Has(SectionCatalog, ActionCreate))
}
