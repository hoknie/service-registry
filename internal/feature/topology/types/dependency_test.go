package types

import (
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestTargetKeys(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("0199a3a4-0000-7000-8000-000000000001")
	ext := "postgres"
	got := TargetKeys([]NewTarget{{ProjectID: &id, Ref: "acme/api"}, {External: &ext, Ref: "external:postgres"}, {Ref: "acme/gone"}})
	want := []string{id.String(), "external:postgres", "ref:acme/gone"}
	if !slices.Equal(got, want) {
		t.Fatalf("keys %v, want %v", got, want)
	}
	if empty := TargetKeys(nil); empty == nil || len(empty) != 0 {
		t.Fatalf("keys of no targets %#v, want an empty non-nil slice", empty)
	}
}
