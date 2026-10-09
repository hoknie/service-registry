package catalog

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
	bad := "old"
	if _, err := ValidateBranchQuery(BranchQuery{State: &bad}); err != InvalidBranchState {
		t.Fatal(err)
	}
	zero := int64(0)
	if _, err := ValidateBranchQuery(BranchQuery{Limit: &zero}); err != InvalidBranchPage || InvalidBranchPage.Code() != "validation.invalid_pagination" {
		t.Fatal(err)
	}
}
