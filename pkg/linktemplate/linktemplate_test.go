package linktemplate

import (
	"errors"
	"testing"
)

func TestParseAndExpand(t *testing.T) {
	p, err := Parse("https://logs.example/{project.slug}?q={query|raw}&e={env|lower}&{{x}}", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Names(); len(got) != 3 || got[0] != "project.slug" || got[2] != "env" {
		t.Fatalf("names: %v", got)
	}
	values := map[string]string{"project.slug": "a b/c", "query": "a=b&c", "env": "PROD"}
	s, missing := p.Expand(func(n string) (string, bool) { v, ok := values[n]; return v, ok })
	if s != "https://logs.example/a%20b%2Fc?q=a=b&c&e=prod&{x}" || len(missing) != 0 {
		t.Fatalf("%q %v", s, missing)
	}
	_, missing = p.Expand(func(string) (string, bool) { return "", false })
	if len(missing) != 3 || missing[0] != "env" {
		t.Fatalf("missing: %v", missing)
	}
}

func TestParseErrors(t *testing.T) {
	for _, c := range []struct {
		template string
		unknown  bool
		pos      int
	}{
		{"a}b", false, 2},
		{"x{name", false, 2},
		{"{Bad}", false, 2},
		{"{name|upper}", false, 7},
		{"{other}", true, 2},
		{"", false, 1},
	} {
		_, err := Parse(c.template, func(n string) bool { return n == "name" })
		var e *Error
		if !errors.As(err, &e) || e.UnknownVariable != c.unknown || e.Pos != c.pos {
			t.Errorf("%q: %v", c.template, err)
		}
	}
}

func TestEscapeKeepsOnlyUnreserved(t *testing.T) {
	if got := Escape("aZ09-._~ /?&é"); got != "aZ09-._~%20%2F%3F%26%C3%A9" {
		t.Fatal(got)
	}
}
