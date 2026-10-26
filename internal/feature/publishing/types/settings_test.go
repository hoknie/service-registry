package types

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
)

func yes() *bool { return new(true) }
func no() *bool  { return new(false) }

func TestMergeAndVersions(t *testing.T) {
	t.Parallel()
	tags := []Version{{Pattern: "v*", Kind: knowledgetypes.RefTag}}
	org := ChainNode{Name: "acme", Kind: "organization", Slug: "acme", Own: NodeSettings{Publish: yes(), Versions: &tags, DefaultBranch: no()}}
	project := ChainNode{Name: "api", Kind: "project"}
	s, from := Merge([]ChainNode{project, org})
	if !s.Publish || len(s.Versions) != 1 || s.DefaultBranch || !s.CurrentRelease || from.Publish != 1 || from.CurrentRelease != -1 {
		t.Fatalf("%+v %+v", s, from)
	}
	for _, c := range []struct {
		kind                 knowledgetypes.RefKind
		name                 string
		isDefault, isRelease bool
		want                 bool
	}{
		{knowledgetypes.RefBranch, "main", true, false, false},
		{knowledgetypes.RefTag, "v1.0.0", false, false, true},
		{knowledgetypes.RefTag, "stable", false, true, true},
		{knowledgetypes.RefTag, "stable", false, false, false},
		{knowledgetypes.RefBranch, "v1", false, false, false},
	} {
		if got := s.VersionAllowed(c.kind, c.name, c.isDefault, c.isRelease); got != c.want {
			t.Errorf("%+v: %v", c, got)
		}
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()
	str := func(s string) *string { return &s }
	bad := []Version{{Pattern: "v*", Kind: "commit"}}
	for _, c := range []struct {
		n    NodeSettings
		kind string
	}{
		{NodeSettings{Name: str("x")}, "organization"},
		{NodeSettings{Name: str("-x")}, "project"},
		{NodeSettings{Name: str(strings.Repeat("a", 64))}, "project"},
		{NodeSettings{Host: str("Docs.acme.com")}, "project"},
		{NodeSettings{Host: str("docs.acme.com:443")}, "project"},
		{NodeSettings{Host: str("registry.example")}, "project"},
		{NodeSettings{Versions: &bad}, "folder"},
	} {
		if _, err := Validate(c.n, c.kind, "registry.example"); !errors.Is(err, InvalidPublication) {
			t.Errorf("%+v: %v", c.n, err)
		}
	}
	n, err := Validate(NodeSettings{Name: str("Acme-API"), Host: str("docs.acme.com")}, "project", "registry.example")
	if err != nil || *n.Name != "acme-api" {
		t.Fatalf("%v %v", n, err)
	}
}

func TestPublicPartAndEntries(t *testing.T) {
	t.Parallel()
	id := func() uuid.UUID { return uuid.Must(uuid.NewV7()) }
	org := Node{ID: id(), Kind: "organization", Slug: "acme", Name: "Acme", Own: NodeSettings{Publish: yes()}}
	backend := Node{ID: id(), ParentID: &org.ID, Kind: "folder", Slug: "backend", Name: "Backend"}
	hr := Node{ID: id(), ParentID: &org.ID, Kind: "folder", Slug: "hr", Name: "HR", Own: NodeSettings{Publish: no()}}
	api := Node{ID: id(), ParentID: &backend.ID, Kind: "project", Slug: "api", Name: "API"}
	payroll := Node{ID: id(), ParentID: &hr.ID, Kind: "project", Slug: "payroll", Name: "Payroll", Own: NodeSettings{Publish: yes()}}
	chain := []ChainNode{{ID: org.ID, Kind: org.Kind, Slug: org.Slug, Own: org.Own}}
	pub, ok := BuildPublic(chain, org, []Node{backend, hr, api, payroll})
	if !ok {
		t.Fatal("published")
	}
	if c := pub.Children(org.ID); len(c) != 1 || c[0].Slug != "backend" {
		t.Fatalf("children %+v", c)
	}
	node, ancestors, docPath, found := pub.Walk([]string{"Backend", "api", "docs", "x.md"})
	if !found || node.ID != api.ID || len(ancestors) != 2 || docPath != "docs/x.md" {
		t.Fatalf("%v %v %q %v", node.Slug, len(ancestors), docPath, found)
	}
	if _, _, _, found := pub.Walk([]string{"hr", "payroll"}); found {
		t.Fatal("a closed folder hides its subtree")
	}
	if p := pub.Projects(org.ID); len(p) != 1 || strings.Join(pub.PathOf(api.ID), "/") != "backend/api" {
		t.Fatalf("%+v", p)
	}
	if _, ok := BuildPublic([]ChainNode{{Kind: "organization"}}, org, nil); ok {
		t.Fatal("unpublished entry")
	}

	name := "acme-api"
	entries := Entries([]ChainNode{
		{Kind: "project", Own: NodeSettings{Name: &name}},
		{Kind: "folder"},
		{Kind: "organization", Slug: "acme", Own: NodeSettings{Publish: yes()}},
	})
	if len(entries) != 2 || entries[0].Segment != "acme-api" || entries[1].Segment != "acme" {
		t.Fatalf("%+v", entries)
	}
	if e := Entries([]ChainNode{{Kind: "project", Own: NodeSettings{Publish: no()}}, {Kind: "organization", Own: NodeSettings{Publish: yes()}}}); len(e) != 0 {
		t.Fatalf("%+v", e)
	}
}

func TestPickVersion(t *testing.T) {
	t.Parallel()
	docs := PublicDocs{Branches: []PublicVersion{{Name: "main", Kind: knowledgetypes.RefBranch}},
		Tags: []PublicVersion{{Name: "v1", Kind: knowledgetypes.RefTag, Release: true}}}
	docs.Default = &docs.Tags[0]
	if v := Pick(docs, knowledgetypes.RefSelector{}); v == nil || v.Name != "v1" {
		t.Fatal("default")
	}
	if v := Pick(docs, knowledgetypes.RefSelector{Kind: knowledgetypes.RefTag, Release: true}); v == nil || v.Name != "v1" {
		t.Fatal("release")
	}
	if v := Pick(docs, knowledgetypes.RefSelector{Kind: knowledgetypes.RefBranch, Name: "main"}); v == nil || v.Kind != knowledgetypes.RefBranch {
		t.Fatal("branch")
	}
	if Pick(docs, knowledgetypes.RefSelector{Kind: knowledgetypes.RefTag, Name: "main"}) != nil {
		t.Fatal("kind matters")
	}
}
