package types

import (
	"errors"
	"strings"
	"testing"
)

func TestDefaultSettingsCollectTheRootReadme(t *testing.T) {
	s := DefaultSettings()
	for p, want := range map[string]bool{
		"README.md": true, "readme": true, "ReadMe.rst": true, "README": true,
		"docs/README.md": false, "README.": false, "READMEs.md": false, "openspec/specs/a/spec.md": false,
	} {
		if got := s.Collects(p); got != want {
			t.Errorf("%s: %v", p, got)
		}
	}
}

func TestDefaultSettingsCollectAPIDescriptions(t *testing.T) {
	s := DefaultSettings()
	for p, want := range map[string]bool{
		"openapi.yaml": true, "api/openapi.v1.json": true, "svc/swagger.yml": true, "proto/pets/v1/pets.proto": true,
		"apidocs/index.html": true, "docs/api-docs/v1/guide.md": true, "openapi.txt": false, "docs/api.yaml": false,
		"apidocs.md": false, "OpenAPI.yaml": true, "ApiDocs/Users.md": true, "proto/Billing.PROTO": true, "Swagger.JSON": true,
	} {
		if got := s.Collects(p); got != want {
			t.Errorf("%s: %v", p, got)
		}
	}
	s.Exclude = []string{"third_party/**"}
	if s.Collects("third_party/google/api/http.proto") {
		t.Error("exclude applies to API descriptions")
	}
	s.APIs = false
	if s.Collects("openapi.yaml") || !s.Collects("README.md") {
		t.Error("apis off leaves only the include rule")
	}
	if (Settings{Include: []string{"docs/**"}, APIs: true}).Collects("README.md") {
		t.Error("the include list still replaces the README rule")
	}
}

func TestPatternsReplaceTheReadmeRule(t *testing.T) {
	s := Settings{Include: []string{"openspec/**", "docs/**/*.md"}, Exclude: []string{"docs/drafts/**"}}
	for p, want := range map[string]bool{
		"openspec/specs/a/b/spec.md": true, "docs/x.md": true, "docs/a/b.md": true, "docs/drafts/x.md": false,
		"docs/x.png": false, "README.md": false, "Openspec/x.md": false,
	} {
		if got := s.Collects(p); got != want {
			t.Errorf("%s: %v", p, got)
		}
	}
	if (Settings{Include: []string{}}).Collects("README.md") {
		t.Error("an empty include collects nothing")
	}
}

func TestBranches(t *testing.T) {
	s := Settings{Branches: []string{"release/*"}}
	for name, want := range map[string]bool{"main": true, "release/1": true, "release/1/x": false, "dev": false} {
		if got := s.CollectsBranch(name, "main"); got != want {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestValidateSettings(t *testing.T) {
	ok, err := ValidateSettings(Settings{Include: nil})
	if err != nil || ok.Exclude == nil || ok.Branches == nil || ok.Include != nil {
		t.Fatalf("%+v %v", ok, err)
	}
	many := make([]string, 51)
	for i := range many {
		many[i] = "a"
	}
	for _, bad := range []Settings{
		{Include: []string{"docs/[a-"}}, {Exclude: []string{""}}, {Branches: []string{strings.Repeat("a", 201)}},
		{Include: many}, {Include: []string{"a\nb"}},
	} {
		if _, err := ValidateSettings(bad); !errors.Is(err, InvalidSettings) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
}

func TestMergeTakesEachFieldFromTheNearestNode(t *testing.T) {
	list := func(s ...string) *[]string { return &s }
	project := ChainNode{Name: "api", Own: NodeSettings{Exclude: list()}}
	folder := ChainNode{Name: "backend", Own: NodeSettings{IncludeSet: true, Include: []string{"docs/**"}, Exclude: list("docs/drafts/**")}}
	org := ChainNode{Name: "acme", Own: NodeSettings{Branches: list("release/*"), IncludeSet: true, Include: []string{"*.md"}}}

	s, from := Merge([]ChainNode{project, folder, org})
	if strings.Join(s.Include, ",") != "docs/**" || len(s.Exclude) != 0 || strings.Join(s.Branches, ",") != "release/*" {
		t.Fatalf("merged: %+v", s)
	}
	if from != (From{Include: 1, Exclude: 0, Branches: 2, APIs: -1}) {
		t.Fatalf("from: %+v", from)
	}

	project.Own = NodeSettings{IncludeSet: true}
	s, from = Merge([]ChainNode{project, folder, org})
	if s.Include != nil || from.Include != 0 || strings.Join(s.Exclude, ",") != "docs/drafts/**" || from.Exclude != 1 {
		t.Fatalf("readme: %+v %+v", s, from)
	}

	s, from = Merge([]ChainNode{{Name: "api"}, {Name: "acme"}})
	if s.Include != nil || s.Exclude == nil || len(s.Exclude) != 0 || s.Branches == nil || from != (From{-1, -1, -1, -1}) || !s.APIs {
		t.Fatalf("defaults: %+v %+v", s, from)
	}
	off := false
	project.Own = NodeSettings{APIs: &off}
	s, from = Merge([]ChainNode{project, folder, org})
	if s.APIs || from.APIs != 0 {
		t.Fatalf("apis: %+v %+v", s, from)
	}
	if !(NodeSettings{}).IsEmpty() || (NodeSettings{IncludeSet: true}).IsEmpty() || (NodeSettings{APIs: &off}).IsEmpty() {
		t.Fatal("IsEmpty")
	}
}

func TestValidateNodeSettingsChecksTheSetFields(t *testing.T) {
	bad := []string{"docs/[a-"}
	for _, n := range []NodeSettings{{IncludeSet: true, Include: bad}, {Exclude: &bad}, {Branches: &[]string{""}}} {
		if _, err := ValidateNodeSettings(n); !errors.Is(err, InvalidSettings) {
			t.Errorf("%+v: %v", n, err)
		}
	}
	if _, err := ValidateNodeSettings(NodeSettings{IncludeSet: true, Exclude: &[]string{}}); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsMayContain(t *testing.T) {
	t.Parallel()
	cases := []struct {
		include []string
		dir     string
		want    bool
	}{
		{[]string{"openspec/**/*.md"}, "openspec", true},
		{[]string{"openspec/**/*.md"}, "openspec/specs/a", true},
		{[]string{"openspec/**/*.md"}, "target", false},
		{[]string{"openspec/**/*.md"}, "target/openspec", false},
		{[]string{"**/*.md"}, "node_modules/x", true},
		{[]string{"*.md"}, "docs", false},
		{[]string{"docs/*.md"}, "docs", true},
		{[]string{"docs/*.md"}, "docs/a", false},
		{[]string{"docs/**"}, "docs/a/b", true},
		{[]string{"d?cs/*/x.md"}, "docs/a", true},
		{[]string{"{docs/a,b}/x.md"}, "c", true},
		{[]string{"{docs,b}/x.md"}, "c", false},
		{[]string{"/openspec/*.md"}, "openspec", true},
		{nil, "docs", false},
		{nil, "", true},
	}
	for _, c := range cases {
		if got := (Settings{Include: c.include}).MayContain(c.dir); got != c.want {
			t.Errorf("%v in %q: got %v, want %v", c.include, c.dir, got, c.want)
		}
	}
}
