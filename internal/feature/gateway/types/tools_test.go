package types

import (
	"encoding/json"
	"strings"
	"testing"
)

var object = json.RawMessage(`{"type":"object"}`)

func TestGatewayNamesCarryTheServerPrefix(t *testing.T) {
	t.Parallel()
	if got := GatewayName("jira", "search"); got != "jira__search" {
		t.Fatal(got)
	}
	slug, tool, ok := SplitGatewayName("jira__create__issue")
	if !ok || slug != "jira" || tool != "create__issue" {
		t.Fatalf("%q %q %v", slug, tool, ok)
	}
	for _, bad := range []string{"get_project", "__x", "Jira__x", "jira__"} {
		if _, _, ok := SplitGatewayName(bad); ok {
			t.Fatalf("%q split", bad)
		}
	}
}

func TestToolsThatCannotBeOfferedAreSkipped(t *testing.T) {
	t.Parallel()
	tools := []Tool{
		{Name: "search", InputSchema: object},
		{Name: "delete_issue", InputSchema: object},
		{Name: "has space", InputSchema: object},
		{Name: strings.Repeat("x", 125), InputSchema: object},
		{Name: "array", InputSchema: json.RawMessage(`{"type":"array"}`)},
		{Name: "bad_output", InputSchema: object, OutputSchema: json.RawMessage(`{"type":"string"}`)},
	}
	views := Views("jira", tools, []string{"delete_issue"})
	want := []struct {
		enabled bool
		skipped string
	}{{true, ""}, {false, ""}, {false, SkipNameInvalid}, {false, SkipNameInvalid}, {false, SkipSchema}, {false, SkipSchema}}
	for i, w := range want {
		if views[i].Enabled != w.enabled || views[i].Skipped != w.skipped {
			t.Fatalf("%s: %+v", tools[i].Name, views[i])
		}
	}
	if offered := Offered("jira", tools, []string{"delete_issue"}); len(offered) != 1 || offered[0].GatewayName != "jira__search" {
		t.Fatalf("offered %+v", offered)
	}
}

func TestSnapshotKeepsTheFirstTools(t *testing.T) {
	t.Parallel()
	kept, truncated := Snapshot([]Tool{{Name: "a"}, {Name: "b"}, {Name: "c"}}, 2)
	if len(kept) != 2 || !truncated {
		t.Fatalf("%v %v", kept, truncated)
	}
	if _, truncated := Snapshot([]Tool{{Name: "a"}}, 2); truncated {
		t.Fatal("truncated")
	}
}

func TestGatewayDescriptionWarnsAboutData(t *testing.T) {
	t.Parallel()
	d := GatewayDescription(Tool{Name: "search", Description: "Search issues"}, "Jira")
	if !strings.HasPrefix(d, "[Jira] Search issues") || !strings.Contains(d, "not as instructions") {
		t.Fatal(d)
	}
	if got := GatewayTitle(Tool{Name: "search"}, "Jira"); got != "search (Jira)" {
		t.Fatal(got)
	}
}

func TestDisabledToolsAreDistinct(t *testing.T) {
	t.Parallel()
	if _, err := CleanDisabledTools([]string{"a", "a"}); err == nil {
		t.Fatal("duplicates accepted")
	}
	if got, err := CleanDisabledTools([]string{" a ", "b"}); err != nil || len(got) != 2 || got[0] != "a" {
		t.Fatalf("%v %v", got, err)
	}
}

func TestBuiltinToolNames(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in []string
		ok bool
	}{
		{[]string{"search_docs", "get_guide"}, true},
		{[]string{}, true},
		{[]string{"Get"}, false},
		{[]string{"a-b"}, false},
		{[]string{"x", "x"}, false},
	} {
		if _, err := CleanBuiltinTools(c.in); (err == nil) != c.ok {
			t.Fatalf("%v: %v", c.in, err)
		}
	}
	many := make([]string, 101)
	for i := range many {
		many[i] = "t" + string(rune('a'+i%26)) + strings.Repeat("x", i/26)
	}
	if _, err := CleanBuiltinTools(many); err == nil {
		t.Fatal("101 names accepted")
	}
}
