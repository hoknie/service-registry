package types

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestRightsCanonical(t *testing.T) {
	t.Parallel()
	in := Rights{
		SectionUsers:  {ActionUpdate, ActionRead, ActionRead},
		Section("x"):  {ActionRead},
		SectionGroups: {Action("fly")},
	}
	want := Rights{SectionUsers: {ActionRead, ActionUpdate}}
	if got := in.Canonical(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Canonical() = %v, want %v", got, want)
	}
	if got := Rights(nil).Canonical(); got == nil || len(got) != 0 {
		t.Fatalf("Canonical() of nil = %#v", got)
	}
}

func TestGroupNameKeys(t *testing.T) {
	t.Parallel()
	got := GroupNameKeys([]string{"Ops", "SRE-Team", "dev"})
	if want := []string{"ops", "sre-team", "dev"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("GroupNameKeys = %v, want %v", got, want)
	}
	if got := GroupNameKeys(nil); got == nil || len(got) != 0 {
		t.Fatalf("GroupNameKeys(nil) = %#v", got)
	}
}

func TestSkillsRightsFollowTheSkillsScope(t *testing.T) {
	t.Parallel()
	rights := Rights{SectionSkills: {ActionCreate, ActionUpdate}}
	if got := rights.ForScopes(Scopes{ScopeRead, ScopeWrite}); got.Has(SectionSkills, ActionCreate) {
		t.Fatalf("write opened skills: %v", got)
	}
	got := rights.ForScopes(Scopes{ScopeSkills})
	if !got.Has(SectionSkills, ActionUpdate) || got.Has(SectionSkills, ActionDelete) {
		t.Fatalf("skills:write: %v", got)
	}
	if !rights.AllowsScope(ScopeSkills) || !ScopeSkills.Global() {
		t.Fatal("skills:write is a global scope allowed by the skills right")
	}
	if _, err := ValidateRights(map[string][]string{"skills": {"read"}}); err == nil {
		t.Fatal("skills read accepted")
	}
}

func TestMCPRightsFollowTheServersScope(t *testing.T) {
	t.Parallel()
	rights := Rights{SectionMCP: {ActionUpdate}}
	if got := rights.ForScopes(Scopes{ScopeRead, ScopeWrite}); got.Has(SectionMCP, ActionUpdate) {
		t.Fatalf("write opened mcp: %v", got)
	}
	if got := rights.ForScopes(Scopes{ScopeMCPServers}); !got.Has(SectionMCP, ActionUpdate) || got.Has(SectionMCP, ActionCreate) {
		t.Fatalf("mcp_servers:write: %v", got)
	}
	if !rights.AllowsScope(ScopeMCPServers) || !ScopeMCPServers.Global() {
		t.Fatal("mcp_servers:write is a global scope allowed by the mcp right")
	}
}

func TestServerSetKeepsNoneApartFromUnlimited(t *testing.T) {
	t.Parallel()
	if s := ServerSetOf(nil); s.Limited || s.Stored() != nil {
		t.Fatalf("nil %+v", s)
	}
	if s := ServerSetOf([]uuid.UUID{}); !s.Limited || s.Stored() == nil || len(s.Stored()) != 0 {
		t.Fatalf("empty %+v", s)
	}
}
