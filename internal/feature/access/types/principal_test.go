package types

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/platform/apperr"
)

func kindOf(err error) apperr.Kind {
	if e, ok := errors.AsType[*apperr.Error](err); ok {
		return e.Kind
	}
	return 0
}

func TestScopesLimitOnlyTokenRequests(t *testing.T) {
	session := Principal{UserID: uuid.Must(uuid.NewV7()), SessionID: uuid.Must(uuid.NewV7())}
	token := func(scopes ...Scope) Principal {
		return Principal{UserID: session.UserID, TokenID: uuid.Must(uuid.NewV7()), Scopes: scopes}
	}
	all := token(AllScopes...)
	nodes := func(scopes ...Scope) Principal {
		p := token(scopes...)
		p.Nodes = []uuid.UUID{uuid.Must(uuid.NewV7())}
		return p
	}
	tests := []struct {
		name string
		p    Principal
		need Need
		want apperr.Kind
	}{
		{"session needs nothing", session, NeedSession, 0},
		{"session global route", session, NeedUsersWrite, 0},
		{"token on a session route", all, NeedSession, apperr.SessionRequired},
		{"any scope reads me", token(ScopeMCP), NeedAny, 0},
		{"read reads", token(ScopeRead), NeedRead, 0},
		{"read does not write", token(ScopeRead), NeedWrite, apperr.InsufficientScope},
		{"write does not read", token(ScopeWrite), NeedRead, apperr.InsufficientScope},
		{"write writes", token(ScopeWrite), NeedWrite, 0},
		{"users do not read the catalog", token(ScopeUsersRead), NeedRead, apperr.InsufficientScope},
		{"users read reads users", token(ScopeUsersRead), NeedUsersRead, 0},
		{"users read does not change users", token(ScopeUsersRead), NeedUsersWrite, apperr.InsufficientScope},
		{"users write does not read users", token(ScopeUsersWrite), NeedUsersRead, apperr.InsufficientScope},
		{"users write changes users", token(ScopeUsersWrite), NeedUsersWrite, 0},
		{"groups read reads groups", token(ScopeGroupsRead), NeedGroupsRead, 0},
		{"groups write changes groups", token(ScopeGroupsWrite), NeedGroupsWrite, 0},
		{"clusters read reads clusters", token(ScopeClustersRead), NeedClustersRead, 0},
		{"clusters write changes clusters", token(ScopeClustersWrite), NeedClustersWrite, 0},
		{"settings read reads settings", token(ScopeSettingsRead), NeedSettingsRead, 0},
		{"settings write changes settings", token(ScopeSettingsWrite), NeedSettingsWrite, 0},
		{"users do not manage groups", token(ScopeUsersRead, ScopeUsersWrite), NeedGroupsRead, apperr.InsufficientScope},
		{"settings do not manage clusters", token(ScopeSettingsWrite), NeedClustersWrite, apperr.InsufficientScope},
		{"catalog root opens no global route", token(ScopeCatalogRoot), NeedSettingsRead, apperr.InsufficientScope},
		{"read and write do not administer", token(ScopeRead, ScopeWrite), NeedUsersRead, apperr.InsufficientScope},
		{"mcp opens no catalog route", token(ScopeMCP), NeedRead, apperr.InsufficientScope},
		{"whole write without nodes", token(ScopeWrite), NeedWriteWhole, 0},
		{"whole write needs write", token(ScopeRead), NeedWriteWhole, apperr.InsufficientScope},
		{"whole write refuses nodes", nodes(ScopeRead, ScopeWrite), NeedWriteWhole, apperr.InsufficientScope},
		{"nodes still write the catalog", nodes(ScopeWrite), NeedWrite, 0},
		{"session writes whole", session, NeedWriteWhole, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kindOf(RequireScope(tt.p, tt.need)); got != tt.want {
				t.Fatalf("kind %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCatalogRootOfTokens(t *testing.T) {
	owner := uuid.Must(uuid.NewV7())
	root, other := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	session := Principal{UserID: owner, SessionID: uuid.Must(uuid.NewV7()), IsSuperadmin: true}
	whole := Principal{UserID: owner, TokenID: uuid.Must(uuid.NewV7()), Scopes: Scopes{ScopeWrite, ScopeCatalogRoot}, IsSuperadmin: true}
	nodes := Principal{UserID: owner, TokenID: uuid.Must(uuid.NewV7()), Scopes: Scopes{ScopeWrite, ScopeCatalogRoot},
		Nodes: []uuid.UUID{root}, CatalogAdmin: true}
	demoted := nodes
	demoted.CatalogAdmin = false
	without := nodes
	without.Scopes = Scopes{ScopeWrite}
	tests := []struct {
		name string
		p    Principal
		node *uuid.UUID
		want apperr.Kind
	}{
		{"superadmin session", session, nil, 0},
		{"user session", Principal{UserID: owner, SessionID: session.SessionID}, nil, apperr.Forbidden},
		{"whole token creates", whole, nil, 0},
		{"whole token moves any root", whole, &other, 0},
		{"whole token without the scope", Principal{UserID: owner, TokenID: whole.TokenID, Scopes: Scopes{ScopeWrite}, IsSuperadmin: true}, nil, apperr.InsufficientScope},
		{"node token creates", nodes, nil, 0},
		{"node token moves its root", nodes, &root, 0},
		{"node token does not move other roots", nodes, &other, apperr.InsufficientScope},
		{"node token of a former superadmin", demoted, nil, apperr.Forbidden},
		{"node token without the scope", without, nil, apperr.InsufficientScope},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kindOf(RequireCatalogRoot(tt.p, tt.node)); got != tt.want {
				t.Fatalf("kind %d, want %d", got, tt.want)
			}
		})
	}
	if !nodes.AddsRoots() || whole.AddsRoots() || demoted.AddsRoots() || without.AddsRoots() {
		t.Fatal("only a node token with catalog.root of a superadmin adds roots")
	}
}
