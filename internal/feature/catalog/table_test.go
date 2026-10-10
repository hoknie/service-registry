package catalog

import (
	"strings"
	"testing"
)

func TestValidateTable(t *testing.T) {
	t.Parallel()
	s := func(v string) *string { return &v }
	cases := []struct {
		name string
		in   TableQuery
		ok   bool
	}{
		{"empty", TableQuery{}, true},
		{"empty strings", TableQuery{Q: s(""), Kind: s(""), Activity: s("")}, true},
		{"all", TableQuery{Q: s("bill"), Kind: s("project"), Labels: []string{"team", "tier=gold"}, Activity: s("failed")}, true},
		{"long q", TableQuery{Q: s(strings.Repeat("я", 101))}, false},
		{"control in q", TableQuery{Q: s("a\nb")}, false},
		{"kind", TableQuery{Kind: s("service")}, false},
		{"label key", TableQuery{Labels: []string{"Team"}}, false},
		{"empty label value", TableQuery{Labels: []string{"team="}}, true},
		{"too many labels", TableQuery{Labels: []string{"a", "b", "c", "d", "e", "f"}}, false},
		{"idle", TableQuery{Activity: s("idle")}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := ValidateTable(c.in)
			if (err == nil) != c.ok {
				t.Fatalf("got %v", err)
			}
			if err != nil && err != InvalidFilter {
				t.Fatalf("got %v", err)
			}
		})
	}
	f, _ := ValidateTable(TableQuery{Labels: []string{"tier=gold", "team"}})
	if f.Labels[0].Key != "tier" || *f.Labels[0].Value != "gold" || f.Labels[1].Value != nil || !f.Active() {
		t.Fatalf("got %+v", f)
	}
}
