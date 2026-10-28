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
		"openapi.yaml": true, "api/openapi.v1.json": true, "svc/swagger.yml": false, "proto/pets/v1/pets.proto": false,
		"apidocs/index.html": false, "docs/api-docs/v1/guide.md": true, "openapi.txt": false, "docs/api.yaml": false,
		"OpenAPI.yaml": true, "Api-Docs/Users.md": true, "Swagger.JSON": false,
	} {
		if got := s.Collects(p); got != want {
			t.Errorf("%s: %v", p, got)
		}
	}
	s.Exclude = []string{"generated/**"}
	if s.Collects("generated/openapi.yaml") {
		t.Error("exclude applies to API descriptions")
	}
	s.APIs = false
	if s.Collects("openapi.yaml") || !s.Collects("README.md") || !s.IsAPIPath("openapi.yaml") {
		t.Error("apis off leaves only the include rule")
	}
	if (Settings{Include: []string{"docs/**"}, APIs: true}).Collects("README.md") {
		t.Error("the include list still replaces the README rule")
	}
}

func TestAPIPathsFromSettings(t *testing.T) {
	s := DefaultSettings()
	s.APIPaths = []string{"**/*.PROTO", "Contracts/**"}
	for p, want := range map[string]bool{
		"proto/pets/v1/pets.proto": true, "api/Billing.Proto": true, "contracts/v1.yaml": true, "CONTRACTS/x.md": true,
		"openapi.yaml": false, "api-docs/index.md": false,
	} {
		if got := s.Collects(p); got != want {
			t.Errorf("%s: %v", p, got)
		}
	}
	s.APIPaths = []string{}
	if s.Collects("openapi.yaml") || s.IsAPIPath("openapi.yaml") {
		t.Error("an empty api_paths collects no API descriptions")
	}
}

func TestDefaultSettingsAreCopies(t *testing.T) {
	s := DefaultSettings()
	s.APIPaths[0] = "x"
	s.RefreshEvents[0] = EventRepoSyncManual
	d := DefaultSettings()
	if d.APIPaths[0] != DefaultAPIPaths[0] || DefaultAPIPaths[0] != "**/openapi*.{yaml,yml,json}" || d.RefreshEvents[0] != EventServiceDeployed {
		t.Fatalf("defaults changed: %+v", d)
	}
	if len(d.RefreshEvents) != 4 || !d.Listens(EventClusterDeployed) || d.Refresh != RefreshInterval {
		t.Fatalf("default events: %+v", d.RefreshEvents)
	}
	for _, e := range DefaultRefreshEvents {
		if !e.Valid() {
			t.Errorf("%s invalid", e)
		}
	}
	if CollectEvent("push").Valid() {
		t.Error("push is not an event")
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
	s := Settings{Refs: []RefPattern{{Pattern: "release/*", Kind: RefBranch}, {Pattern: "dev", Kind: RefTag}}}
	for name, want := range map[string]bool{"main": true, "release/1": true, "release/1/x": false, "dev": false} {
		if got := s.CollectsBranch(name, "main"); got != want {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestValidateSettings(t *testing.T) {
	ok, err := ValidateSettings(Settings{Include: nil})
	if err != nil || ok.Exclude == nil || ok.Refs == nil || ok.Include != nil {
		t.Fatalf("%+v %v", ok, err)
	}
	many := make([]string, 51)
	for i := range many {
		many[i] = "a"
	}
	for _, bad := range []Settings{
		{Include: []string{"docs/[a-"}}, {Exclude: []string{""}}, {Refs: []RefPattern{{Pattern: strings.Repeat("a", 201), Kind: RefBranch}}},
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
	org := ChainNode{Name: "acme", Own: NodeSettings{Refs: &[]RefPattern{{Pattern: "release/*", Kind: RefBranch}}, IncludeSet: true,
		Include: []string{"*.md"}}}

	s, from := Merge([]ChainNode{project, folder, org})
	if strings.Join(s.Include, ",") != "docs/**" || len(s.Exclude) != 0 || len(s.Refs) != 1 || s.Refs[0].Pattern != "release/*" {
		t.Fatalf("merged: %+v", s)
	}
	if from != (From{Include: 1, Exclude: 0, Refs: 2, APIs: -1, APIPaths: -1, Refresh: -1, RefreshEvents: -1}) {
		t.Fatalf("from: %+v", from)
	}

	project.Own = NodeSettings{IncludeSet: true}
	s, from = Merge([]ChainNode{project, folder, org})
	if s.Include != nil || from.Include != 0 || strings.Join(s.Exclude, ",") != "docs/drafts/**" || from.Exclude != 1 {
		t.Fatalf("readme: %+v %+v", s, from)
	}

	s, from = Merge([]ChainNode{{Name: "api"}, {Name: "acme"}})
	if s.Include != nil || s.Exclude == nil || len(s.Exclude) != 0 || s.Refs == nil || from != (From{-1, -1, -1, -1, -1, -1, -1}) || !s.APIs || s.Refresh != RefreshInterval {
		t.Fatalf("defaults: %+v %+v", s, from)
	}
	off := false
	project.Own = NodeSettings{APIs: &off}
	s, from = Merge([]ChainNode{project, folder, org})
	if s.APIs || from.APIs != 0 {
		t.Fatalf("apis: %+v %+v", s, from)
	}
	event, interval := RefreshEvent, RefreshInterval
	org.Own = NodeSettings{Refresh: &event}
	project.Own = NodeSettings{}
	s, from = Merge([]ChainNode{project, folder, org})
	if s.Refresh != RefreshEvent || from.Refresh != 2 {
		t.Fatalf("inherited refresh: %+v %+v", s, from)
	}
	project.Own = NodeSettings{Refresh: &interval}
	s, from = Merge([]ChainNode{project, folder, org})
	if s.Refresh != RefreshInterval || from.Refresh != 0 {
		t.Fatalf("own refresh: %+v %+v", s, from)
	}
	if !(NodeSettings{}).IsEmpty() || (NodeSettings{IncludeSet: true}).IsEmpty() || (NodeSettings{APIs: &off}).IsEmpty() ||
		(NodeSettings{Refresh: &event}).IsEmpty() || (NodeSettings{APIPaths: list()}).IsEmpty() ||
		(NodeSettings{RefreshEvents: &[]CollectEvent{}}).IsEmpty() {
		t.Fatal("IsEmpty")
	}
}

func TestMergeAPIPathsAndRefreshEvents(t *testing.T) {
	list := func(s ...string) *[]string { return &s }
	events := func(e ...CollectEvent) *[]CollectEvent { return &e }
	project := ChainNode{Name: "api"}
	folder := ChainNode{Name: "backend"}
	org := ChainNode{Name: "acme", Own: NodeSettings{APIPaths: list("**/*.proto"), RefreshEvents: events(EventRepoSyncWebhook)}}

	s, from := Merge([]ChainNode{project, folder, org})
	if strings.Join(s.APIPaths, ",") != "**/*.proto" || from.APIPaths != 2 || len(s.RefreshEvents) != 1 || from.RefreshEvents != 2 {
		t.Fatalf("inherited: %+v %+v", s, from)
	}
	if !s.Listens(EventRepoSyncWebhook) || s.Listens(EventServiceDeployed) {
		t.Fatalf("listens: %+v", s.RefreshEvents)
	}
	folder.Own = NodeSettings{APIPaths: list("**/*.proto", "spec/**")}
	project.Own = NodeSettings{RefreshEvents: &[]CollectEvent{}}
	s, from = Merge([]ChainNode{project, folder, org})
	if strings.Join(s.APIPaths, ",") != "**/*.proto,spec/**" || from.APIPaths != 1 || len(s.RefreshEvents) != 0 || s.RefreshEvents == nil ||
		from.RefreshEvents != 0 || s.Listens(EventRepoSyncWebhook) {
		t.Fatalf("overridden: %+v %+v", s, from)
	}
	project.Own = NodeSettings{APIPaths: &[]string{}}
	s, _ = Merge([]ChainNode{project, folder, org})
	if len(s.APIPaths) != 0 || s.Collects("proto/a.proto") {
		t.Fatalf("empty paths: %+v", s)
	}
}

func TestValidateNodeSettingsChecksTheSetFields(t *testing.T) {
	bad := []string{"docs/[a-"}
	hourly := Refresh("hourly")
	for _, n := range []NodeSettings{{IncludeSet: true, Include: bad}, {Exclude: &bad}, {Refs: &[]RefPattern{{Pattern: "", Kind: RefBranch}}}, {Refresh: &hourly},
		{APIPaths: &[]string{"api/[a-"}}, {APIPaths: &[]string{""}}, {RefreshEvents: &[]CollectEvent{"push"}},
		{RefreshEvents: &[]CollectEvent{EventRepoSyncWebhook, EventRepoSyncWebhook}}} {
		if _, err := ValidateNodeSettings(n); !errors.Is(err, InvalidSettings) {
			t.Errorf("%+v: %v", n, err)
		}
	}
	for _, n := range []NodeSettings{{IncludeSet: true, Exclude: &[]string{}}, {APIPaths: &[]string{}}, {RefreshEvents: &[]CollectEvent{}},
		{APIPaths: &[]string{"**/*.proto"}, RefreshEvents: &[]CollectEvent{EventRepoSyncManual, EventServiceDeployed}}} {
		if _, err := ValidateNodeSettings(n); err != nil {
			t.Fatalf("%+v: %v", n, err)
		}
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
