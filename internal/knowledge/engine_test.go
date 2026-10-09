package knowledge

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestChunkSplitsByHeadingsAndLength(t *testing.T) {
	doc := "# One\nalpha beta\n\n# Two\n" + strings.Repeat("word ", 100)
	spans := Chunk(doc, 120)
	if len(spans) < 3 {
		t.Fatalf("spans %v", spans)
	}
	if got := Slice(doc, spans[0]); !strings.HasPrefix(got, "# One") || strings.Contains(got, "# Two") {
		t.Fatalf("first %q", got)
	}
	for _, s := range spans {
		if s.End-s.Start > 120 || s.End <= s.Start {
			t.Fatalf("span %v", s)
		}
	}
	if Chunk("", 100) != nil || len(Chunk("  \n ", 100)) != 0 {
		t.Fatal("empty")
	}
	ru := "Репозиторий хранит настройки"
	if got := Slice(ru, Chunk(ru, 200)[0]); got != ru {
		t.Fatalf("runes %q", got)
	}
}

func TestFuseRRFMergesRanks(t *testing.T) {
	p := uuid.Must(uuid.NewV7())
	h := func(path string, snippet string) Hit {
		return Hit{ProjectID: p, Branch: "main", Path: path, Snippet: []Segment{{Text: snippet}}}
	}
	text := []Hit{h("a.md", "text-a"), h("b.md", "text-b")}
	semantic := []Hit{h("b.md", "sem-b"), h("c.md", "sem-c")}
	got := FuseRRF(text, semantic, 10)
	if len(got) != 3 || got[0].Path != "b.md" {
		t.Fatalf("%+v", got)
	}
	if got[0].Snippet[0].Text != "text-b" {
		t.Fatal("the text snippet wins for a file found both ways")
	}
	if len(FuseRRF(text, semantic, 1)) != 1 {
		t.Fatal("limit")
	}
}

func TestParseModeAndOffers(t *testing.T) {
	if m, ok := ParseMode("hybrid"); !ok || m != ModeHybrid {
		t.Fatal("hybrid")
	}
	if _, ok := ParseMode("vector"); ok {
		t.Fatal("unknown mode")
	}
}
