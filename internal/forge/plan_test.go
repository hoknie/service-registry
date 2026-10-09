package forge

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"API": "api", "core_api.v2": "core-api-v2", "--Ünïcode  Repo--": "n-code-repo", "___": "repo",
		"a" + string(make([]byte, 0)) + "b": "ab",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	long := Slugify("x123456789-123456789-123456789-123456789-123456789-123456789-abcdef")
	if len(long) > 63 || long[len(long)-1] == '-' {
		t.Errorf("%q", long)
	}
}

func TestSubgroups(t *testing.T) {
	if got := Subgroups("acme", "acme/platform/billing/api", true); !reflect.DeepEqual(got, []string{"platform", "billing"}) {
		t.Error(got)
	}
	if got := Subgroups("ACME", "acme/api", true); got != nil {
		t.Error(got)
	}
	if got := Subgroups("acme", "acme/platform/api", false); got != nil {
		t.Error(got)
	}
	if got := GroupPath("acme", []string{"platform", "billing"}, 1); got != "acme/platform" {
		t.Error(got)
	}
}

func TestKeepFilters(t *testing.T) {
	s := Settings{NameInclude: []string{"svc-*"}, NameExclude: []string{"*-old"}}
	for name, want := range map[string]bool{"svc-api": true, "SVC-Web": true, "svc-api-old": false, "tools": false} {
		if got := Keep(s, RemoteRepo{Name: name}); got != want {
			t.Errorf("%s: %v", name, got)
		}
	}
	if Keep(Settings{}, RemoteRepo{Name: "x", Archived: true}) || !Keep(Settings{IncludeArchived: true}, RemoteRepo{Name: "x", Archived: true}) {
		t.Error("archived")
	}
	if Keep(Settings{}, RemoteRepo{Name: "x", Fork: true}) || !Keep(Settings{IncludeForks: true}, RemoteRepo{Name: "x", Fork: true}) {
		t.Error("forks")
	}
}

func TestPlanPairsByExternalIDAndFindsOrphans(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	links := []Link{{ProjectID: a, ExternalID: "1", FullPath: "acme/api"}, {ProjectID: b, ExternalID: "2"}, {ProjectID: c, ExternalID: "3", Orphaned: true}}
	remotes := []RemoteRepo{{ExternalID: "1", FullPath: "acme/core-api"}, {ExternalID: "3"}, {ExternalID: "4"}, {ExternalID: "4"}}
	targets, orphans := Plan(remotes, links)
	if len(targets) != 3 || targets[0].Link.ProjectID != a || targets[1].Link.ProjectID != c || targets[2].Link != nil {
		t.Fatalf("%+v", targets)
	}
	if len(orphans) != 1 || orphans[0].ProjectID != b {
		t.Fatalf("%+v", orphans)
	}
}

func TestDetailsOnlyForChanges(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	if !NeedsDetails(Target{Remote: RemoteRepo{PushedAt: &t0}}) {
		t.Error("new")
	}
	same := Target{Remote: RemoteRepo{PushedAt: &t0}, Link: &Link{SourceUpdatedAt: &t0}}
	if NeedsDetails(same) {
		t.Error("unchanged")
	}
	moved := Target{Remote: RemoteRepo{PushedAt: &t0, UpdatedAt: &t1}, Link: &Link{SourceUpdatedAt: &t0}}
	if !NeedsDetails(moved) {
		t.Error("changed")
	}
	back := Target{Remote: RemoteRepo{PushedAt: &t0}, Link: &Link{SourceUpdatedAt: &t0, Orphaned: true}}
	if !NeedsDetails(back) {
		t.Error("returning")
	}
}

func TestPercentages(t *testing.T) {
	got := Percentages(map[string]float64{"Go": 750, "Shell": 249, "Makefile": 1})
	if got["Go"] != 75 || got["Shell"] != 24.9 || got["Makefile"] != 0.1 {
		t.Fatal(got)
	}
	if len(Percentages(map[string]float64{})) != 0 {
		t.Fatal("empty")
	}
}

func TestBranchFilter(t *testing.T) {
	all := BranchFilter(Settings{}, "main")
	if !all("feature/x") || !all("main") {
		t.Error("empty patterns take every branch")
	}
	rel := BranchFilter(Settings{BranchInclude: []string{"release/*"}}, "main")
	for name, want := range map[string]bool{"main": true, "release/1": true, "feature/a": false, "Release/1": false} {
		if got := rel(name); got != want {
			t.Errorf("%s: %v", name, got)
		}
	}
}
