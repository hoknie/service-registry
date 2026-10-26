package types

import (
	"errors"
	"slices"
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
	creator := Rights{SectionCatalog: {ActionCreate}}
	bySession := Principal{UserID: owner, SessionID: session.SessionID, Rights: creator}
	byToken := Principal{UserID: owner, TokenID: whole.TokenID, Scopes: Scopes{ScopeWrite, ScopeCatalogRoot}, Rights: creator}
	byNodes := byToken
	byNodes.Nodes = []uuid.UUID{root}
	tests := []struct {
		name   string
		p      Principal
		node   *uuid.UUID
		action Action
		want   apperr.Kind
	}{
		{"right creates from a session", bySession, nil, ActionCreate, 0},
		{"right does not delete", bySession, &root, ActionDelete, apperr.Forbidden},
		{"right creates with a token", byToken, nil, ActionCreate, 0},
		{"right does not move with a token", byToken, &root, ActionUpdate, apperr.Forbidden},
		{"right creates with a node token", byNodes, nil, ActionCreate, 0},
		{"superadmin session", session, nil, ActionCreate, 0},
		{"user session", Principal{UserID: owner, SessionID: session.SessionID}, nil, ActionCreate, apperr.Forbidden},
		{"whole token creates", whole, nil, ActionCreate, 0},
		{"whole token moves any root", whole, &other, ActionUpdate, 0},
		{"whole token without the scope", Principal{UserID: owner, TokenID: whole.TokenID, Scopes: Scopes{ScopeWrite}, IsSuperadmin: true}, nil, ActionCreate, apperr.InsufficientScope},
		{"node token creates", nodes, nil, ActionCreate, 0},
		{"node token moves its root", nodes, &root, ActionUpdate, 0},
		{"node token does not move other roots", nodes, &other, ActionUpdate, apperr.InsufficientScope},
		{"node token of a former superadmin", demoted, nil, ActionCreate, apperr.Forbidden},
		{"node token without the scope", without, nil, ActionCreate, apperr.InsufficientScope},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := kindOf(RequireCatalogRoot(tt.p, tt.node, tt.action)); got != tt.want {
				t.Fatalf("kind %d, want %d", got, tt.want)
			}
		})
	}
	if !nodes.AddsRoots() || whole.AddsRoots() || demoted.AddsRoots() || without.AddsRoots() || !byNodes.AddsRoots() || byToken.AddsRoots() {
		t.Fatal("only a node token with catalog.root of a superadmin or of a creator adds roots")
	}
}

func TestRequireRightAndScopeIntersection(t *testing.T) {
	t.Parallel()
	ops := Rights{SectionClusters: {ActionRead, ActionUpdate}, SectionScans: {ActionRead}, SectionTokens: {ActionRead}}
	for _, tt := range []struct {
		name    string
		p       Principal
		section Section
		action  Action
		ok      bool
	}{
		{"superadmin", Principal{IsSuperadmin: true}, SectionUsers, ActionCreate, true},
		{"held right", Principal{Rights: ops}, SectionClusters, ActionUpdate, true},
		{"other action", Principal{Rights: ops}, SectionClusters, ActionDelete, false},
		{"other section", Principal{Rights: ops}, SectionSecrets, ActionRead, false},
		{"no rights", Principal{}, SectionScans, ActionRead, false},
	} {
		err := RequireRight(tt.p, tt.section, tt.action)
		if (err == nil) != tt.ok || (err != nil && kindOf(err) != apperr.Forbidden) {
			t.Errorf("%s: %v", tt.name, err)
		}
	}
	got := ops.ForScopes(Scopes{ScopeClustersRead, ScopeSettingsWrite})
	if len(got) != 1 || !got.Has(SectionClusters, ActionRead) || got.Has(SectionClusters, ActionUpdate) || got.Has(SectionTokens, ActionRead) {
		t.Errorf("ForScopes = %v", got)
	}
	for _, tt := range []struct {
		scope Scope
		ok    bool
	}{{ScopeClustersRead, true}, {ScopeClustersWrite, true}, {ScopeSettingsRead, true}, {ScopeSettingsWrite, false}, {ScopeUsersRead, false}} {
		if ops.AllowsScope(tt.scope) != tt.ok {
			t.Errorf("AllowsScope(%s) != %v", tt.scope, tt.ok)
		}
	}
	token := Principal{TokenID: uuid.Must(uuid.NewV7()), IsSuperadmin: true, Scopes: Scopes{ScopeUsersRead}}
	if r := token.EffectiveRights(); len(r) != 1 || !r.Has(SectionUsers, ActionRead) {
		t.Errorf("superadmin token rights = %v", r)
	}
	if r := (Principal{IsSuperadmin: true}).EffectiveRights(); len(r) != len(Sections) {
		t.Errorf("superadmin rights = %v", r)
	}
}

func TestValidateRights(t *testing.T) {
	t.Parallel()
	got, err := ValidateRights(map[string][]string{"clusters": {"update", "read"}, "users": {}})
	if err != nil || len(got) != 1 || got[SectionClusters][0] != ActionRead || got[SectionClusters][1] != ActionUpdate {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, bad := range []map[string][]string{
		{"scans": {"delete"}}, {"billing": {"read"}}, {"clusters": {"read", "read"}}, {"link_kinds": {"read"}}, {"users": {"remove"}},
	} {
		if _, err := ValidateRights(bad); !errors.Is(err, InvalidPermissions) {
			t.Errorf("%v: %v", bad, err)
		}
	}
}

func TestTokenActsPerActionOfOwner(t *testing.T) {
	t.Parallel()
	for _, section := range Sections {
		pair, ok := sectionScopes[section]
		if !ok {
			continue
		}
		for _, held := range SectionActions[section] {
			owner := Rights{section: {held}}
			for _, scopes := range []Scopes{{pair[0]}, {pair[1]}, {pair[0], pair[1]}} {
				p := Principal{TokenID: uuid.Must(uuid.NewV7()), Scopes: scopes, Rights: owner.ForScopes(scopes)}
				for _, action := range SectionActions[section] {
					want := action == held && scopes.Has(pair[scopeHalf(action)])
					if got := RequireRight(p, section, action) == nil; got != want {
						t.Errorf("%s: owner holds %s, scopes %v, %s allowed = %v, want %v", section, held, scopes, action, got, want)
					}
				}
			}
		}
	}
}

func TestCatalogScopeIssuedByAnyCatalogRight(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		rights Rights
		ok     bool
	}{
		{Rights{SectionCatalog: {ActionCreate}}, true},
		{Rights{SectionCatalog: {ActionDelete}}, true},
		{Rights{SectionCatalog: {ActionRead}}, false},
		{Rights{SectionClusters: {ActionCreate}}, false},
		{nil, false},
	} {
		if got := (Grantor{Rights: tt.rights}).mayIssue(ScopeCatalogRoot); got != tt.ok {
			t.Errorf("mayIssue(catalog.root) with %v = %v, want %v", tt.rights, got, tt.ok)
		}
	}
}

func TestCatalogReadFollowsTheReadScope(t *testing.T) {
	t.Parallel()
	held := Rights{SectionCatalog: {ActionRead, ActionUpdate}}
	for _, tt := range []struct {
		name   string
		scopes Scopes
		want   []Action
	}{
		{"read scope keeps read", Scopes{ScopeRead}, []Action{ActionRead}},
		{"catalog.root keeps the writes", Scopes{ScopeCatalogRoot}, []Action{ActionUpdate}},
		{"both keep both", Scopes{ScopeRead, ScopeCatalogRoot}, []Action{ActionRead, ActionUpdate}},
		{"mcp keeps nothing", Scopes{ScopeMCP}, nil},
	} {
		if got := held.ForScopes(tt.scopes)[SectionCatalog]; !slices.Equal(got, tt.want) {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
	if _, err := ValidateRights(map[string][]string{"catalog": {"read"}}); err != nil {
		t.Errorf("catalog read refused: %v", err)
	}
}

func TestRequireMetrics(t *testing.T) {
	t.Parallel()
	token := uuid.Must(uuid.NewV7())
	cases := []struct {
		name string
		p    Principal
		ok   bool
	}{
		{"superadmin", Principal{IsSuperadmin: true}, true},
		{"scans read", Principal{Rights: Rights{SectionScans: {ActionRead}}}, true},
		{"secrets read", Principal{Rights: Rights{SectionSecrets: {ActionRead}}}, true},
		{"clusters read", Principal{Rights: Rights{SectionClusters: {ActionRead}}}, false},
		{"secrets create", Principal{Rights: Rights{SectionSecrets: {ActionCreate}}}, false},
		{"no rights", Principal{}, false},
		{"superadmin token read", Principal{TokenID: token, IsSuperadmin: true, Scopes: Scopes{ScopeRead}}, false},
		{"superadmin token settings", Principal{TokenID: token, IsSuperadmin: true, Scopes: Scopes{ScopeSettingsRead}}, true},
		{"user token settings without right", Principal{TokenID: token, Scopes: Scopes{ScopeSettingsRead}, Rights: Rights{SectionClusters: {ActionRead}}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := RequireMetrics(c.p)
			if (err == nil) != c.ok {
				t.Fatalf("err = %v, want ok %v", err, c.ok)
			}
			if err != nil && kindOf(err) != apperr.Forbidden {
				t.Fatalf("kind = %v", kindOf(err))
			}
		})
	}
}

func TestPublicationRightAndScope(t *testing.T) {
	t.Parallel()
	owner := uuid.Must(uuid.NewV7())
	tok := func(nodes []uuid.UUID, s ...Scope) Principal {
		return Principal{UserID: owner, TokenID: uuid.Must(uuid.NewV7()), Scopes: s, Nodes: nodes}
	}
	if err := RequireScope(tok(nil, ScopeRead, ScopePublication), NeedPublicationWrite); err != nil {
		t.Fatal(err)
	}
	for _, p := range []Principal{tok(nil, ScopeRead, ScopeWrite), tok(nil, ScopePublication), tok([]uuid.UUID{owner}, ScopeRead, ScopePublication)} {
		if err := RequireScope(p, NeedPublicationWrite); apperr.From(err) == nil || apperr.From(err).Kind != apperr.InsufficientScope {
			t.Errorf("%v: %v", p.Scopes, err)
		}
	}
	if err := RequireScope(Principal{UserID: owner}, NeedPublicationWrite); err != nil {
		t.Fatal("sessions are not limited by scopes")
	}
	rights, err := ValidateRights(map[string][]string{"publication": {"update"}})
	if err != nil || !rights.Has(SectionPublication, ActionUpdate) || !rights.AllowsScope(ScopePublication) {
		t.Fatalf("%v %v", rights, err)
	}
	if _, err := ValidateRights(map[string][]string{"publication": {"read"}}); !errors.Is(err, InvalidPermissions) {
		t.Fatal(err)
	}
	if got := rights.ForScopes(Scopes{ScopeRead}); got.Has(SectionPublication, ActionUpdate) {
		t.Fatal("read does not open publication")
	}
}
