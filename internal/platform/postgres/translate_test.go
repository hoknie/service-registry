package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"svc-registry/internal/platform/apperr"
)

func TestTranslate(t *testing.T) {
	t.Parallel()
	taken := apperr.Conflicting("conflict.slug_taken", "taken")
	constraints := Constraints{"nodes_parent_slug_key": taken, "nodes_parent_id_fkey": apperr.ErrNotFound}
	for name, tc := range map[string]struct {
		err  error
		want error
		kind apperr.Kind
	}{
		"code passes":        {err: taken, want: taken},
		"wrapped code":       {err: fmt.Errorf("insert: %w", taken), want: taken},
		"constraint":         {err: &pgconn.PgError{ConstraintName: "nodes_parent_slug_key"}, want: taken},
		"foreign key":        {err: &pgconn.PgError{ConstraintName: "nodes_parent_id_fkey"}, want: apperr.ErrNotFound},
		"unknown constraint": {err: &pgconn.PgError{ConstraintName: "other_key"}, kind: apperr.Internal},
		"database down":      {err: context.DeadlineExceeded, want: apperr.ErrUnavailable},
		"anything else":      {err: errors.New("boom"), kind: apperr.Internal},
	} {
		got := Translate(tc.err, constraints)
		if tc.want != nil && !errors.Is(got, tc.want) {
			t.Errorf("%s: Translate = %v, want %v", name, got, tc.want)
		}
		if tc.kind != 0 && apperr.From(got).Kind != tc.kind {
			t.Errorf("%s: kind = %v, want %v", name, apperr.From(got).Kind, tc.kind)
		}
	}
	if Translate(nil, constraints) != nil {
		t.Error("nil must stay nil")
	}
}
