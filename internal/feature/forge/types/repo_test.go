package types

import (
	"strings"
	"testing"
	"time"
)

func TestStoredRepoWithoutDetailsKeepsStoredDetails(t *testing.T) {
	t.Parallel()
	got := RepoRecord{Remote: RemoteRepo{FullPath: "a/b"}}.Stored()
	if got.WithDetails || got.DefaultBranch != nil || got.License != nil || got.ChangedAt != nil || got.Readme != nil {
		t.Fatalf("%+v", got)
	}
	if got.Topics == nil || len(got.Topics) != 0 || got.Languages == nil || len(got.Languages) != 0 {
		t.Fatalf("empty collections expected: %+v", got)
	}
}

func TestStoredRepoWithDetails(t *testing.T) {
	t.Parallel()
	pushed, updated := time.Unix(200, 0), time.Unix(100, 0)
	readme := "# x"
	cases := map[string]struct {
		remote  string
		details *string
		want    *string
	}{
		"details license wins":        {"MIT", new("Apache-2.0"), new("Apache-2.0")},
		"remote license kept":         {"MIT", nil, new("MIT")},
		"too long details is ignored": {"MIT", new(strings.Repeat("x", MaxLicenseLen+1)), new("MIT")},
		"no license":                  {"", new(""), nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := RepoRecord{
				Remote:  RemoteRepo{DefaultBranch: "main", License: tc.remote, PushedAt: &pushed, UpdatedAt: &updated},
				Details: &Details{Readme: &readme, Truncated: true, License: tc.details},
			}.Stored()
			if !got.WithDetails || got.Readme != &readme || got.Truncated == nil || !*got.Truncated {
				t.Fatalf("%+v", got)
			}
			if got.DefaultBranch == nil || *got.DefaultBranch != "main" || got.ChangedAt == nil || !got.ChangedAt.Equal(pushed) {
				t.Fatalf("%+v", got)
			}
			if (got.License == nil) != (tc.want == nil) || (got.License != nil && *got.License != *tc.want) {
				t.Fatalf("license %v, want %v", got.License, tc.want)
			}
		})
	}
}

func TestRepositoryFromForge(t *testing.T) {
	t.Parallel()
	got := Repository{}.FromForge()
	if got.Origin != OriginForge || got.Topics == nil || got.Languages == nil {
		t.Fatalf("%+v", got)
	}
	kept := Repository{Topics: []string{"go"}, Languages: map[string]float64{"Go": 1}}.FromForge()
	if len(kept.Topics) != 1 || kept.Languages["Go"] != 1 {
		t.Fatalf("%+v", kept)
	}
}

func TestRunResultStoredBoundsProblems(t *testing.T) {
	t.Parallel()
	if got := (RunResult{}).Stored().Problems; got == nil || len(got) != 0 {
		t.Fatalf("%v", got)
	}
	many := make([]Problem, MaxProblems+5)
	if got := (RunResult{Problems: many}).Stored().Problems; len(got) != MaxProblems {
		t.Fatalf("%d", len(got))
	}
	few := []Problem{{FullPath: "a", Code: "c"}}
	if got := (RunResult{Problems: few}).Stored().Problems; len(got) != 1 {
		t.Fatalf("%v", got)
	}
}
