package types

import "testing"

func TestBranchNameDropsRefsHeads(t *testing.T) {
	for in, want := range map[string]string{"refs/heads/feature/x": "feature/x", " main ": "main", "release/1.0": "release/1.0"} {
		if got, err := BranchName(in); err != nil || got != want {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "refs/heads/", "-x", "a b", "a\tb"} {
		if _, err := BranchName(bad); err != InvalidBranch {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestBranchQuery(t *testing.T) {
	f, err := ValidateBranchQuery(BranchQuery{})
	if err != nil || f.State != BranchesActive || f.Limit != 50 || f.Offset != 0 {
		t.Fatalf("%+v %v", f, err)
	}
	gone, prefix := "gone", "feature/"
	if f, _ := ValidateBranchQuery(BranchQuery{State: &gone, Prefix: &prefix}); f.State != BranchesGone || f.Prefix != prefix {
		t.Fatalf("%+v", f)
	}
	if _, err := ValidateBranchQuery(BranchQuery{State: new("old")}); err != InvalidBranchState {
		t.Fatal(err)
	}
	if _, err := ValidateBranchQuery(BranchQuery{Limit: new(int64(0))}); err != InvalidBranchPage || InvalidBranchPage.Code != "validation.invalid_pagination" {
		t.Fatal(err)
	}
}

func TestBranchSHANormalizes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   *string
		want *string
	}{
		{nil, nil},
		{new("ABCDEF1"), new("abcdef1")},
		{new("abc"), nil},
		{new("not-a-sha"), nil},
		{new(""), nil},
	}
	for _, c := range cases {
		got := BranchSHA(c.in)
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%v: got %v want %v", c.in, got, c.want)
		}
	}
}

func TestHeadsMoved(t *testing.T) {
	t.Parallel()
	known := map[string]string{"main": "abcdef1"}
	cases := []struct {
		name     string
		branches []RepoSyncBranch
		want     bool
	}{
		{"same head", []RepoSyncBranch{{Name: "main", HeadSHA: "ABCDEF1"}}, false},
		{"moved head", []RepoSyncBranch{{Name: "main", HeadSHA: "abcdef2"}}, true},
		{"new branch", []RepoSyncBranch{{Name: "dev", HeadSHA: "abcdef1"}}, true},
		{"invalid head", []RepoSyncBranch{{Name: "dev", HeadSHA: "zzz"}}, false},
		{"none", nil, false},
	}
	for _, c := range cases {
		if got := HeadsMoved(known, c.branches); got != c.want {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}
