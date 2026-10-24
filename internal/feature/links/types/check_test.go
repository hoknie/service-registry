package types

import (
	"slices"
	"testing"
)

func TestSightingsDeduplicateURLsAndPairs(t *testing.T) {
	t.Parallel()
	a, b := "https://b.example", "https://a.example"
	got := Sightings([]Link{
		{LinkKey: "logs", URL: &a},
		{LinkKey: "logs", URL: &a},
		{LinkKey: "traces", URL: &a},
		{LinkKey: "logs", URL: &b},
		{LinkKey: "missing"},
	})
	if !slices.Equal(got.URLs, []string{b, a}) {
		t.Fatalf("urls %v", got.URLs)
	}
	want := []TargetPair{{URL: a, LinkKey: "logs"}, {URL: a, LinkKey: "traces"}, {URL: b, LinkKey: "logs"}}
	if !slices.Equal(got.Pairs, want) {
		t.Fatalf("pairs %v", got.Pairs)
	}
	if empty := Sightings(nil); len(empty.URLs) != 0 || len(empty.Pairs) != 0 {
		t.Fatalf("%+v", empty)
	}
}
