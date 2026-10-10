package knowledge

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
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
		fmt.Errorf("%w: 768 vs 384", ErrEngineDimensions):            "search.engine_dimensions",
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

func TestScanExtendsOnlyAContinuousSeries(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	main := []ScanBranch{{Name: "main", Commit: "abc"}}
	last := ScanTail{Status: ScanUnchanged, Trigger: TriggerSchedule, Branches: main, FinishedAt: at}
	next := Scan{Status: ScanUnchanged, Trigger: TriggerSchedule, Branches: []ScanBranch{{Name: "main", Commit: "abc"}},
		StartedAt: at.Add(5 * time.Minute), FinishedAt: at.Add(5*time.Minute + 1200*time.Millisecond)}
	gap := 10 * time.Minute
	cases := []struct {
		name string
		edit func(s *Scan, l *ScanTail)
		want bool
	}{
		{"same series", func(*Scan, *ScanTail) {}, true},
		{"overlapping clocks", func(s *Scan, _ *ScanTail) { s.StartedAt = at.Add(-time.Second) }, true},
		{"long pause", func(s *Scan, _ *ScanTail) { s.StartedAt = at.Add(12 * time.Hour) }, false},
		{"manual after schedule", func(s *Scan, _ *ScanTail) { s.Trigger = TriggerManual }, false},
		{"other commit", func(s *Scan, _ *ScanTail) { s.Branches = []ScanBranch{{Name: "main", Commit: "def"}} }, false},
		{"last was ok", func(_ *Scan, l *ScanTail) { l.Status = ScanOK }, false},
		{"run was ok", func(s *Scan, _ *ScanTail) { s.Status = ScanOK }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s, l := next, last
			c.edit(&s, &l)
			if got := s.Extends(l, gap); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
	if next.DurationMS() != 1200 {
		t.Fatalf("duration %d", next.DurationMS())
	}
}

func TestIndexFailureKeepsCodeDetailAndChain(t *testing.T) {
	err := fmt.Errorf("%w: embeddings API answered with Bearer secret-token", ErrEmbeddingsUnavailable)
	f := NewIndexFailure(err)
	if f.Code != "search.embeddings_unavailable" || strings.Contains(f.Detail, "secret-token") || !errors.Is(f, ErrSearchUnavailable) {
		t.Fatalf("%+v", f)
	}
}
