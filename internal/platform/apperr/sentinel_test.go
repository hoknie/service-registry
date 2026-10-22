package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestSharedSentinelsMatchByKind(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		err    error
		target error
		want   bool
	}{
		"not found itself":            {ErrNotFound, ErrNotFound, true},
		"any not found":               {New(NotFound), ErrNotFound, true},
		"wrapped not found":           {fmt.Errorf("lookup: %w", New(NotFound)), ErrNotFound, true},
		"unavailable database":        {UnavailableDB(), ErrUnavailable, true},
		"not found is not missing db": {ErrNotFound, ErrUnavailable, false},
	} {
		if got := errors.Is(tc.err, tc.target); got != tc.want {
			t.Errorf("%s: errors.Is = %v, want %v", name, got, tc.want)
		}
	}
}

func TestCodesMatchByIdentityThroughWrap(t *testing.T) {
	t.Parallel()
	taken := Conflicting("conflict.slug_taken", "a sibling with this slug already exists")
	other := Conflicting("conflict.slug_taken", "a sibling with this slug already exists")
	if !errors.Is(Wrap(taken), taken) {
		t.Fatal("a code must survive Wrap")
	}
	if errors.Is(taken, other) {
		t.Fatal("two declared codes are different errors")
	}
	if code, message := Wrap(taken).(*Error).Public(); code != "conflict.slug_taken" || message != "a sibling with this slug already exists" {
		t.Fatalf("Public() = %q %q", code, message)
	}
	if code, _ := Wrap(ErrUnavailable).(*Error).Public(); code != "service_unavailable" {
		t.Fatalf("unavailable code = %q", code)
	}
}
