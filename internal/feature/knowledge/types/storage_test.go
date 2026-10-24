package types

import (
	"crypto/sha256"
	"testing"
	"time"

	forgetypes "svc-registry/internal/feature/forge/types"
)

func TestNewSnapshotTotalsCountStoredAndSkipped(t *testing.T) {
	t.Parallel()
	files := []File{{Path: "a.md", Bytes: 5}, {Path: "b.md", Bytes: 7}, {Path: "c.png", Bytes: 9, Skip: "binary"}}
	tests := []struct {
		name string
		snap NewSnapshot
		want SnapshotTotals
	}{
		{"collected", NewSnapshot{Status: StatusPartial, Files: files}, SnapshotTotals{Stored: 2, Bytes: 12, Skipped: 1}},
		{"failed", NewSnapshot{Status: StatusFailed, Files: files}, SnapshotTotals{}},
		{"empty", NewSnapshot{Status: StatusOK}, SnapshotTotals{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.snap.Totals(); got != tt.want {
				t.Fatalf("Totals() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNewSnapshotSameTreeComparesStoredKeys(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256([]byte("alpha"))
	files := []File{{Path: "a.md", SHA256: sum}, {Path: "b.png", Skip: "binary", Meta: Meta{Git: GitUntracked}}}
	snap := NewSnapshot{Files: files}
	same := map[string]string{"a.md": files[0].TreeKey(), "b.png": "skip:binary:untracked"}
	if files[0].TreeKey() != "8ed3f6ad685b959ead7022518e1af76cd816f8e8ec7ccdda1ed4018e8f2223f8:" {
		t.Fatalf("TreeKey() = %q", files[0].TreeKey())
	}
	tests := []struct {
		name   string
		stored map[string]string
		want   bool
	}{
		{"same", same, true},
		{"fewer", map[string]string{"a.md": same["a.md"]}, false},
		{"changed content", map[string]string{"a.md": "00:", "b.png": same["b.png"]}, false},
		{"other path", map[string]string{"a.md": same["a.md"], "c.png": same["b.png"]}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := snap.SameTree(tt.stored); got != tt.want {
				t.Fatalf("SameTree() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeSettingsStoredDropsUnsetInclude(t *testing.T) {
	t.Parallel()
	if got := (NodeSettings{Include: []string{"x"}}).Stored(); got.Include != nil {
		t.Fatalf("Include = %v, want nil", got.Include)
	}
	if got := (NodeSettings{IncludeSet: true, Include: []string{"x"}}).Stored(); len(got.Include) != 1 {
		t.Fatalf("Include = %v, want kept", got.Include)
	}
}

func TestScanStoredFillsEmptyCollections(t *testing.T) {
	t.Parallel()
	got := Scan{}.Stored()
	if got.Branches == nil || got.Warnings == nil || got.Notes == nil || got.Counts == nil {
		t.Fatalf("Stored() = %+v, want empty non-nil collections", got)
	}
	kept := Scan{Warnings: []string{"w"}}.Stored()
	if len(kept.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want kept", kept.Warnings)
	}
}

func TestNewSourceRepoRecordDefaults(t *testing.T) {
	t.Parallel()
	pushed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	readme := "# hi"
	license := "MIT"
	bare := NewSourceRepoRecord(forgetypes.RemoteRepo{FullPath: "a/b"}, nil)
	if bare.Repo.Topics == nil || bare.Languages == nil || bare.Repo.Visibility != forgetypes.VisibilityPublic ||
		bare.DefaultBranch != nil || bare.License != nil || bare.Details || bare.ChangedAt != nil || bare.Truncated != nil {
		t.Fatalf("bare record = %+v", bare)
	}
	full := NewSourceRepoRecord(forgetypes.RemoteRepo{FullPath: "a/b", DefaultBranch: "main", License: "GPL", Visibility: forgetypes.VisibilityPrivate,
		PushedAt: &pushed}, &forgetypes.Details{Readme: &readme, Truncated: true, License: &license, Languages: map[string]float64{"Go": 1}})
	if !full.Details || *full.DefaultBranch != "main" || *full.License != "MIT" || full.Repo.Visibility != forgetypes.VisibilityPrivate ||
		full.Readme != &readme || !*full.Truncated || full.Languages["Go"] != 1 || full.ChangedAt == nil {
		t.Fatalf("full record = %+v", full)
	}
}
