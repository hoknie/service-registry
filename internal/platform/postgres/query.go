package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
)

func Struct[R, T any](to func(R) T) pgx.RowToFunc[T] {
	return func(row pgx.CollectableRow) (T, error) {
		r, err := pgx.RowToStructByName[R](row)
		if err != nil {
			var zero T
			return zero, err
		}
		return to(r), nil
	}
}

func List[T any](ctx context.Context, q pgtx.Querier, c Constraints, to pgx.RowToFunc[T], sql string, args ...any) ([]T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, Translate(err, c)
	}
	out, err := pgx.CollectRows(rows, to)
	if err != nil {
		return nil, Translate(err, c)
	}
	return out, nil
}

func One[T any](ctx context.Context, q pgtx.Querier, c Constraints, to pgx.RowToFunc[T], sql string, args ...any) (T, error) {
	out, err := firstRow(ctx, q, to, sql, args)
	return out, Translate(err, c)
}

func Find[T any](ctx context.Context, q pgtx.Querier, c Constraints, to pgx.RowToFunc[T], sql string, args ...any) (*T, error) {
	out, err := firstRow(ctx, q, to, sql, args)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, Translate(err, c)
	}
	return &out, nil
}

func firstRow[T any](ctx context.Context, q pgtx.Querier, to pgx.RowToFunc[T], sql string, args []any) (T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		var zero T
		return zero, err
	}
	return pgx.CollectOneRow(rows, to)
}

func Exec(ctx context.Context, q pgtx.Querier, c Constraints, sql string, args ...any) (int64, error) {
	tag, err := q.Exec(ctx, sql, args...)
	if err != nil {
		return 0, Translate(err, c)
	}
	return tag.RowsAffected(), nil
}

func ExecFound(ctx context.Context, q pgtx.Querier, c Constraints, sql string, args ...any) error {
	n, err := Exec(ctx, q, c, sql, args...)
	if err == nil && n == 0 {
		return apperr.ErrNotFound
	}
	return err
}

func Pairs[K comparable, V any](ctx context.Context, q pgtx.Querier, c Constraints, sql string, args ...any) (map[K]V, error) {
	found, err := List(ctx, q, c, func(r pgx.CollectableRow) (pair[K, V], error) {
		var p pair[K, V]
		err := r.Scan(&p.key, &p.value)
		return p, err
	}, sql, args...)
	if err != nil {
		return nil, err
	}
	out := make(map[K]V, len(found))
	for _, p := range found {
		out[p.key] = p.value
	}
	return out, nil
}

func Set[K comparable](ctx context.Context, q pgtx.Querier, c Constraints, sql string, args ...any) (map[K]bool, error) {
	found, err := List(ctx, q, c, pgx.RowTo[K], sql, args...)
	if err != nil {
		return nil, err
	}
	out := make(map[K]bool, len(found))
	for _, k := range found {
		out[k] = true
	}
	return out, nil
}

type pair[K, V any] struct {
	key   K
	value V
}
