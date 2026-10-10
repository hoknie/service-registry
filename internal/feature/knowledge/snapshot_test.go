package knowledge

import (
	"errors"
	"strings"
	"testing"
)

func TestCollectAppliesLimitsInPathOrder(t *testing.T) {
	content := map[string]string{
		"docs/a.md": "alpha", "docs/b.md": strings.Repeat("b", 2000), "docs/c.png": "\x89PNG\x00",
		"docs/d.md": "delta", "docs/e.md": "echo", "other.txt": "x",
	}
	var entries []Entry
	for p := range content {
		size := int64(len(content[p]))
		if p == "docs/d.md" {
			size = -1
		}
		entries = append(entries, Entry{Path: p, BlobSHA: "g-" + p, Size: size})
	}
	fetched := map[string]bool{}
	files, err := Collect(entries, Settings{Include: []string{"docs/**"}},
		Limits{MaxFileBytes: 1024, MaxFiles: 3, MaxSnapshotBytes: 9}, func(e Entry, limit int64) ([]byte, error) {
			fetched[e.Path] = true
			return []byte(content[e.Path]), nil
		})
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, f := range files {
		got = append(got, f.Path+":"+string(f.Skip))
	}
	want := "docs/a.md:,docs/b.md:too_large,docs/c.png:binary,docs/d.md:limit,docs/e.md:"
	if strings.Join(got, ",") != want {
		t.Fatalf("%s", strings.Join(got, ","))
	}
	if fetched["docs/b.md"] || fetched["other.txt"] {
		t.Fatalf("fetched %v", fetched)
	}
	if SnapshotStatus(files, false) != StatusPartial || SnapshotStatus(files[:1], false) != StatusOK || SnapshotStatus(files[:1], true) != StatusPartial {
		t.Fatal("status")
	}
	if files[0].SHA256 == ([32]byte{}) || string(files[0].Content) != "alpha" || files[1].Content != nil {
		t.Fatal("content")
	}
}

func TestCollectStopsAtTheFileCount(t *testing.T) {
	entries := []Entry{{Path: "a.md", Size: 1}, {Path: "b.md", Size: 1}, {Path: "c.md", Size: 1}}
	files, _ := Collect(entries, Settings{Include: []string{"*.md"}}, Limits{MaxFileBytes: 10, MaxFiles: 2, MaxSnapshotBytes: 100},
		func(Entry, int64) ([]byte, error) { return []byte("x"), nil })
	if files[2].Skip != SkipLimit || files[1].Skip != "" {
		t.Fatalf("%+v", files)
	}
	boom := errors.New("boom")
	if _, err := Collect(entries, Settings{Include: []string{"*.md"}}, Limits{MaxFileBytes: 10, MaxFiles: 2, MaxSnapshotBytes: 100},
		func(Entry, int64) ([]byte, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}

func TestTreeComparesContentAndSkips(t *testing.T) {
	a := []File{{Path: "x", SHA256: [32]byte{1}}, {Path: "y", Skip: SkipBinary}}
	b := []File{{Path: "x", SHA256: [32]byte{1}}, {Path: "y", Skip: SkipBinary}}
	if Tree(a)["x"] != Tree(b)["x"] || Tree(a)["y"] != "skip:binary" {
		t.Fatal("tree")
	}
}

func TestClassifyAndParse(t *testing.T) {
	cases := map[string]Kind{
		"openspec/specs/catalog/branches/spec.md": KindSpec, "openspec/changes/x/proposal.md": KindChange,
		"openspec/changes/archive/2026-x/tasks.md": KindChange, "openspec/decisions/0036-a-b.md": KindADR,
		"openspec/decisions/README.md": KindDoc, "README.md": KindDoc, "openspec/changes/archive": KindDoc,
	}
	for p, want := range cases {
		if got, _ := Classify(p); got != want {
			t.Errorf("%s: %s", p, got)
		}
	}
	kind, meta := Classify("openspec/specs/catalog/branches/spec.md")
	meta = Parse(kind, "", "# catalog/branches\n\n## Purpose\nBranches of projects.\n\n## Requirements\n\n### Requirement: One\ntext\n### Requirement: Two\n", meta)
	if meta.Capability != "catalog/branches" || meta.Purpose != "Branches of projects." || len(meta.Requirements) != 2 || meta.Requirements[1] != "Two" {
		t.Fatalf("%+v", meta)
	}
	_, meta = Classify("openspec/changes/archive/2026-10-09-x/design.md")
	if meta.Change != "2026-10-09-x" || !meta.Archived {
		t.Fatalf("%+v", meta)
	}
	kind, meta = Classify("openspec/decisions/0036-branches.md")
	meta = Parse(kind, "", "# ADR-0036: Branches\n\n- **Status:** Accepted\n- **Supersedes:** none\nStatus: later\n", meta)
	if meta.Number != 36 || meta.Title != "ADR-0036: Branches" || meta.Status != "Accepted" || meta.Supersedes != "none" {
		t.Fatalf("%+v", meta)
	}
	if !IsText([]byte("привет")) || IsText([]byte{0xff, 0xfe}) || IsText([]byte("a\x00b")) {
		t.Fatal("text")
	}
}
