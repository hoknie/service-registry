package types

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestValidateSearch(t *testing.T) {
	s, err := ValidateSearch(SearchInput{Q: "  cache  "})
	if err != nil || s.Q != "cache" || s.Limit != 20 || s.Offset != 0 {
		t.Fatalf("%+v %v", s, err)
	}
	s, err = ValidateSearch(SearchInput{Q: "x", Limit: new(5), Cursor: Cursor(40)})
	if err != nil || s.Limit != 5 || s.Offset != 40 {
		t.Fatalf("%+v %v", s, err)
	}
	zero, big := 0, 51
	for _, in := range []SearchInput{{Q: ""}, {Q: " "}, {Q: strings.Repeat("a", 201)}, {Q: "x", Limit: &zero},
		{Q: "x", Limit: &big}, {Q: "x", Cursor: "nope"}, {Q: "x", Cursor: Cursor(-1)}} {
		if _, err := ValidateSearch(in); !errors.Is(err, InvalidSearch) {
			t.Errorf("%+v: %v", in, err)
		}
	}
}

func TestSegmentsAndPathWords(t *testing.T) {
	got := Segments("the " + MarkStart + "cache" + MarkStop + " key " + MarkStart + "TTL" + MarkStop)
	want := []Segment{{Text: "the "}, {Text: "cache", Match: true}, {Text: " key "}, {Text: "TTL", Match: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
	if len(Segments("")) != 0 || !reflect.DeepEqual(Segments("plain"), []Segment{{Text: "plain"}}) {
		t.Fatal("plain")
	}
	if !reflect.DeepEqual(PathWords(`Decisions "ADR" or -draft`), []string{"decisions", "adr"}) {
		t.Fatal(PathWords(`Decisions "ADR" or -draft`))
	}
}

func TestPrefixQuery(t *testing.T) {
	for _, c := range []struct {
		q, pos, neg string
		ok          bool
	}{
		{"Репозит", "'репозит':*", "", true},
		{"deploy -репозит", "'deploy':*", "'репозит':*", true},
		{"a ci/cd", "'ci':* & 'cd':*", "", true},
		{`"exact phrase"`, "", "", false},
		{"one or two", "", "", false},
		{"-only", "", "'only':*", true},
	} {
		pos, neg, ok := PrefixQuery(c.q)
		if pos != c.pos || neg != c.neg || ok != c.ok {
			t.Errorf("%q: %q %q %v", c.q, pos, neg, ok)
		}
	}
}

func TestExcludedWords(t *testing.T) {
	if got := ExcludedWords(`deploy -ветки -"draft" ok`); got != "ветки or draft" {
		t.Fatal(got)
	}
	if ExcludedWords("plain words") != "" {
		t.Fatal("no exclusions")
	}
}

func TestSnippetTrimsSegmentsToLimit(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("я", SnippetRunes+10)
	got := Snippet("ab" + MarkStart + long + MarkStop + "tail")
	if len(got) != 2 || got[0].Text != "ab" || !got[1].Match || utf8.RuneCountInString(got[1].Text) != SnippetRunes-2 {
		t.Fatalf("Snippet() = %+v", got)
	}
	if got := Snippet(""); got == nil || len(got) != 0 {
		t.Fatalf("Snippet(\"\") = %#v, want empty", got)
	}
}

func TestNewTextQueryAndPage(t *testing.T) {
	t.Parallel()
	tq := NewTextQuery("Deploy guide -draft")
	if len(tq.PathWords) != 2 || tq.Prefix != "'deploy':* & 'guide':*" || tq.Negative != "'draft':*" || tq.Excluded != "draft" {
		t.Fatalf("NewTextQuery() = %+v", tq)
	}
	q := Search{Limit: 2}
	if q.Probe().Limit != 3 {
		t.Fatalf("Probe().Limit = %d", q.Probe().Limit)
	}
	hits, more := q.Page([]Hit{{Path: "a"}, {Path: "b"}, {Path: "c"}})
	if len(hits) != 2 || !more {
		t.Fatalf("Page() = %v %v", hits, more)
	}
	if hits, more := q.Page([]Hit{{Path: "a"}}); len(hits) != 1 || more {
		t.Fatalf("Page() = %v %v", hits, more)
	}
}
