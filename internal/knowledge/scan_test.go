package knowledge

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestValidateScanFilter(t *testing.T) {
	f, err := ValidateScanFilter(ScanQuery{Kind: "collect", Status: "failed, warning", Trigger: "manual",
		Project: "01a12037-855f-717a-a7bf-eaf02f3867cc"})
	if err != nil || *f.Kind != CollectScan || len(f.Statuses) != 2 || f.Statuses[1] != ScanWarning || *f.Trigger != TriggerManual || f.Project == nil {
		t.Fatalf("%+v %v", f, err)
	}
	for _, q := range []ScanQuery{{Kind: "sync"}, {Status: "ok,broken"}, {Trigger: "cron"}, {Project: "acme"}} {
		if _, err := ValidateScanFilter(q); err != InvalidScanFilter {
			t.Fatalf("%+v: %v", q, err)
		}
	}
	if f, err := ValidateScanFilter(ScanQuery{}); err != nil || f.Kind != nil || f.Statuses != nil {
		t.Fatalf("empty %+v %v", f, err)
	}
}

func TestRedactDetail(t *testing.T) {
	in := "GET https://bot:s3cret@git.example/api?private_token=abc123&x=1: 401 Bearer xyz svcp_Ab3xY9qZ token ghp_zzz"
	out := RedactDetail(in)
	for _, leak := range []string{"s3cret", "abc123", "xyz", "Ab3xY9qZ", "ghp_zzz"} {
		if strings.Contains(out, leak) {
			t.Fatalf("%q leaks %q", out, leak)
		}
	}
	if !strings.Contains(out, "401") || !strings.Contains(out, "git.example") {
		t.Fatalf("lost context: %q", out)
	}
	if r := []rune(RedactDetail(strings.Repeat("я", 2000))); len(r) != ScanDetailLimit {
		t.Fatalf("length %d", len(r))
	}
}

func TestIndexFailureCode(t *testing.T) {
	cases := map[error]string{
		fmt.Errorf("%w: got 1024 want 768", ErrEmbeddingsDimensions): "search.embeddings_dimensions",
		fmt.Errorf("%w: 503", ErrEmbeddingsUnavailable):              "search.embeddings_unavailable",
		fmt.Errorf("%w: qdrant", ErrEngineUnavailable):               "search.engine_unavailable",
		errors.New("boom"): "internal",
	}
	for err, want := range cases {
		if got := IndexFailureCode(err); got != want {
			t.Fatalf("%v: %s, want %s", err, got, want)
		}
		if want != "internal" && !errors.Is(err, ErrSearchUnavailable) {
			t.Fatalf("%v must stay a search unavailability", err)
		}
	}
}

func TestSettle(t *testing.T) {
	s := Scan{Branches: []ScanBranch{{Result: BranchUnchanged}}}
	s.Settle()
	if s.Status != ScanUnchanged {
		t.Fatal(s.Status)
	}
	s.Warn(WarnNoFilesMatched)
	s.Warn(WarnNoFilesMatched)
	s.Settle()
	if s.Status != ScanWarning || len(s.Warnings) != 1 {
		t.Fatal(s.Status, s.Warnings)
	}
	s.Branches = append(s.Branches, ScanBranch{Result: BranchCollected})
	s.Settle()
	if s.Status != ScanWarning {
		t.Fatal("an empty snapshot is not a success", s.Status)
	}
	s.Branches = append(s.Branches, ScanBranch{Result: BranchCollected, Files: 3})
	s.Settle()
	if s.Status != ScanOK {
		t.Fatal(s.Status)
	}
	s.Fail("source.unreadable", errors.New("permission denied"))
	s.Fail("other", nil)
	s.Settle()
	if s.Status != ScanFailed || s.Error.Code != "source.unreadable" {
		t.Fatal(s.Status, s.Error)
	}
}
