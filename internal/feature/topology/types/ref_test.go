package types

import (
	"strings"
	"testing"
)

func TestParseRef(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in, want string
		ok       bool
	}{
		{"ACME/Backend/Billing", "acme/backend/billing", true},
		{" /acme/api/ ", "acme/api", true},
		{"external: Postgres-Main", "external:postgres-main", true},
		{"EXTERNAL:redis", "external:redis", true},
		{"0199a3a4-0000-7000-8000-000000000001", "0199a3a4-0000-7000-8000-000000000001", true},
		{"external:", "", false},
		{"external:-bad", "", false},
		{"external:" + strings.Repeat("a", 64), "", false},
		{"", "", false},
		{"   ", "", false},
		{"acme//api", "", false},
		{"acme/a b", "", false},
		{strings.Repeat("a", 1025), "", false},
	} {
		r, err := ParseRef(c.in)
		if (err == nil) != c.ok || (c.ok && r.Text() != c.want) {
			t.Errorf("%q: %v %q", c.in, err, r.Text())
		}
	}
}

func TestParseRefsDropsRepeatsAndCountsInvalid(t *testing.T) {
	t.Parallel()
	refs, invalid := ParseRefs([]string{"a/b", "A/B", "external:x", "external:"})
	if len(refs) != 2 || invalid != 1 {
		t.Fatal(refs, invalid)
	}
}

func TestParseManifest(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in      string
		refs    int
		problem bool
		err     bool
	}{
		{"dependencies: [a/b, \"external:s3\"]\nowners: x\n", 2, false, false},
		{"name: x\n", 0, false, false},
		{"", 0, false, false},
		{"dependencies: 42\n", 0, false, true},
		{"- a\n", 0, false, true},
		{"dependencies: [\n", 0, false, true},
		{"dependencies: [a/b, 7, \"external:\"]\n", 1, true, false},
		{"dependencies: [" + strings.Repeat("a/b,", 60) + "]\n", 1, true, false},
		{strings.Repeat("#", MaxManifestBytes+1), 0, false, true},
	} {
		m, err := ParseManifest([]byte(c.in))
		if (err != nil) != c.err || len(m.Refs) != c.refs || (m.Problem != "") != c.problem {
			t.Errorf("%.40q: %v %d %q", c.in, err, len(m.Refs), m.Problem)
		}
	}
}

func TestClusterValue(t *testing.T) {
	t.Parallel()
	if got := strings.Join(ClusterValue(" a/b , external:kafka,, "), "|"); got != "a/b|external:kafka" {
		t.Fatal(got)
	}
	if got := len(ClusterValue(strings.Repeat("x,", 100))); got != MaxRefs {
		t.Fatal(got)
	}
	if got := len(strings.Join(ClusterValue(strings.Repeat("y", 5000)), "")); got != MaxClusterValue {
		t.Fatal(got)
	}
}

func TestValidNote(t *testing.T) {
	t.Parallel()
	if n, ok := ValidNote(new("  line\nnext ")); !ok || *n != "line\nnext" {
		t.Fatal(n, ok)
	}
	if n, ok := ValidNote(new(" ")); !ok || n != nil {
		t.Fatal(n, ok)
	}
	if _, ok := ValidNote(new("a\tb")); ok {
		t.Fatal("tab")
	}
	if _, ok := ValidNote(new(strings.Repeat("я", 501))); ok {
		t.Fatal("long")
	}
}
