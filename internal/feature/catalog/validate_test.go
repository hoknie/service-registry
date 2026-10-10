package catalog

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestSlugsAreNormalizedAndBounded(t *testing.T) {
	for raw, want := range map[string]string{" Payments ": "payments", "a-1": "a-1", strings.Repeat("a", 63): strings.Repeat("a", 63)} {
		if got, err := ValidateSlug(raw); err != nil || got != want {
			t.Errorf("ValidateSlug(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, bad := range []string{"", "-bad-", "bad-", "a_b", "a b", "é", strings.Repeat("a", 64)} {
		if _, err := ValidateSlug(bad); err != InvalidSlug {
			t.Errorf("ValidateSlug(%q) = %v, want InvalidSlug", bad, err)
		}
	}
}

func TestNamesAndDescriptions(t *testing.T) {
	if got, err := ValidateName("  API "); err != nil || got != "API" {
		t.Errorf("ValidateName = %q, %v", got, err)
	}
	if _, err := ValidateName(" "); err != InvalidNodeName {
		t.Errorf("blank name: %v", err)
	}
	if _, err := ValidateDescription("line\nline\ttab"); err != nil {
		t.Errorf("newline and tab: %v", err)
	}
	if _, err := ValidateDescription("a\a"); err != InvalidDescription {
		t.Errorf("bell: %v", err)
	}
	if _, err := ValidateDescription(strings.Repeat("x", 2001)); err != InvalidDescription {
		t.Errorf("too long: %v", err)
	}
}

func TestLabelsRules(t *testing.T) {
	ok := []Labels{{"team": "payments", "critical": ""}, {"k8s.io_x-y": "v"}}
	for _, l := range ok {
		if _, err := ValidateLabels(LabelsInput{Labels: l}); err != nil {
			t.Errorf("%v: %v", l, err)
		}
	}
	many := Labels{}
	for i := range 33 {
		many[fmt.Sprintf("k%d", i)] = ""
	}
	bad := []LabelsInput{
		{Labels: Labels{"Team": "x"}},
		{Labels: Labels{"-x": ""}},
		{Labels: Labels{"x": strings.Repeat("v", 64)}},
		{Labels: many},
		{Err: InvalidLabels},
	}
	for _, l := range bad {
		if _, err := ValidateLabels(l); err != InvalidLabels {
			t.Errorf("%v: %v, want InvalidLabels", l, err)
		}
	}
	if got, _ := ValidateLabels(LabelsInput{}); got == nil {
		t.Error("validated labels must never be nil")
	}
}

func TestRepositoryFields(t *testing.T) {
	if f, err := ValidateForge("forgejo"); err != nil || *f != ForgeForgejo {
		t.Errorf("forgejo: %v %v", f, err)
	}
	if f, err := ValidateForge(""); err != nil || f != nil {
		t.Errorf("empty forge: %v %v", f, err)
	}
	if _, err := ValidateForge("svn"); err != InvalidForge {
		t.Errorf("svn: %v", err)
	}
	for _, good := range []string{"https://git.example.com/acme/api", "http://localhost:3000/x"} {
		if u, err := ValidateRepoURL(good); err != nil || u == nil {
			t.Errorf("%s: %v", good, err)
		}
	}
	for _, bad := range []string{"ftp://example.com/x", "https://", "https:///x", "https://a b", "example.com"} {
		if _, err := ValidateRepoURL(bad); err != InvalidRepoURL {
			t.Errorf("%s: %v", bad, err)
		}
	}
	if b, err := ValidateBranch("main"); err != nil || *b != "main" {
		t.Errorf("main: %v", err)
	}
	if b, err := ValidateBranch(""); err != nil || b != nil {
		t.Errorf("empty branch: %v %v", b, err)
	}
	for _, bad := range []string{"-x", "a b"} {
		if _, err := ValidateBranch(bad); err != InvalidBranch {
			t.Errorf("%q: %v", bad, err)
		}
	}
	raw := RepoInput{RepoURL: ptr("https://example.com/x")}
	if _, err := ValidateRepo(KindFolder, raw); err != InvalidRepoFieldsNotAllowed {
		t.Errorf("folder: %v", err)
	}
	if _, err := ValidateRepo(KindProject, raw); err != nil {
		t.Errorf("project: %v", err)
	}
}

func TestRepoChangesSetAndClear(t *testing.T) {
	var c NodeChanges
	err := ValidateRepoChanges(KindProject, RepoInput{Forge: ptr("github"), DefaultBranch: ptr("")}, &c)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Forge.Set || *c.Forge.Value != ForgeGithub || c.RepoURL.Set || !c.DefaultBranch.Set || c.DefaultBranch.Value != nil {
		t.Errorf("changes: %+v", c)
	}
	if err := ValidateRepoChanges(KindOrganization, RepoInput{Forge: ptr("")}, &c); err != InvalidRepoFieldsNotAllowed {
		t.Errorf("organization: %v", err)
	}
}

func TestNestingRules(t *testing.T) {
	org, folder, project := KindOrganization, KindFolder, KindProject
	if !org.MayBeUnder(nil) || folder.MayBeUnder(nil) || project.MayBeUnder(nil) {
		t.Error("top level holds organizations only")
	}
	for _, k := range []NodeKind{org, folder, project} {
		if !k.MayBeUnder(&org) || k.MayBeUnder(&project) {
			t.Errorf("%s under organization / project", k)
		}
	}
	if org.MayBeUnder(&folder) || !folder.MayBeUnder(&folder) || !project.MayBeUnder(&folder) {
		t.Error("folder holds folders and projects")
	}
}

func TestDepthAndGrace(t *testing.T) {
	if d, err := ValidateDepth(nil); err != nil || d != 3 {
		t.Errorf("default depth: %d %v", d, err)
	}
	ten := int64(10)
	if d, err := ValidateDepth(&ten); err != nil || d != 10 {
		t.Errorf("10: %d %v", d, err)
	}
	for _, bad := range []int64{0, 11, -1} {
		if _, err := ValidateDepth(&bad); err != InvalidDepth {
			t.Errorf("depth %d: %v", bad, err)
		}
	}
	if g, err := ValidateGrace(nil); err != nil || g != 86_400 {
		t.Errorf("default grace: %d %v", g, err)
	}
	if g, err := ValidateGrace(ptr[int64](0)); err != nil || g != 0 {
		t.Errorf("0: %d %v", g, err)
	}
	for _, bad := range []int64{-1, 604_801} {
		if _, err := ValidateGrace(&bad); !errors.Is(err, InvalidGracePeriod) {
			t.Errorf("grace %d: %v", bad, err)
		}
	}
}

func TestRolePermissionSets(t *testing.T) {
	set := func(r Role) []string {
		var out []string
		for _, p := range AllPermissions {
			if r.Allows(p) {
				out = append(out, p.String())
			}
		}
		return out
	}
	if got := set(RoleViewer); !reflect.DeepEqual(got, []string{"catalog.read"}) {
		t.Errorf("viewer: %v", got)
	}
	if got := set(RoleEditor); !reflect.DeepEqual(got, []string{"catalog.read", "catalog.write", "catalog.keys"}) {
		t.Errorf("editor: %v", got)
	}
	if got := set(RoleAdmin); len(got) != 4 {
		t.Errorf("admin: %v", got)
	}
	if !(RoleViewer < RoleEditor && RoleEditor < RoleAdmin) {
		t.Error("roles are ordered")
	}
	for _, name := range []string{"viewer", "editor", "admin"} {
		if r, ok := ParseRole(name); !ok || r.String() != name {
			t.Errorf("round trip %s", name)
		}
	}
}
