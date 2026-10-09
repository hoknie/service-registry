package deploy

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/deploy"
)

type EnvironmentStore struct{ pool *pgxpool.Pool }

func NewEnvironmentStore(pool *pgxpool.Pool) *EnvironmentStore { return &EnvironmentStore{pool: pool} }

func (s *EnvironmentStore) List(ctx context.Context) ([]domain.Environment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.key, e.names, e.position, rfc3339(e.created_at) AS created_at,
			rfc3339(e.updated_at) AS updated_at
		FROM environments e
		ORDER BY e.position, e.key`)
	if err != nil {
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[environmentRow])
	if err != nil {
		return nil, dbErr(err)
	}
	out := make([]domain.Environment, 0, len(items))
	for _, r := range items {
		out = append(out, r.environment())
	}
	return out, nil
}

func (s *EnvironmentStore) Insert(ctx context.Context, e domain.NewEnvironment) (domain.Environment, error) {
	rows, err := s.pool.Query(ctx, `
		INSERT INTO environments AS e (id, key, names, position)
		VALUES ($1, $2, $3, $4)
		RETURNING e.id, e.key, e.names, e.position, rfc3339(e.created_at) AS created_at,
			rfc3339(e.updated_at) AS updated_at`, e.ID, e.Key, e.Names, e.Position)
	if err != nil {
		return domain.Environment{}, dbErr(err)
	}
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[environmentRow])
	if err != nil {
		return domain.Environment{}, dbErr(err)
	}
	return r.environment(), nil
}

func (s *EnvironmentStore) Update(ctx context.Context, key string, c domain.EnvironmentChanges) (*domain.Environment, error) {
	var names any
	if c.Names != nil {
		names = c.Names
	}
	rows, err := s.pool.Query(ctx, `
		UPDATE environments AS e
		SET names = COALESCE($2, e.names), position = COALESCE($3, e.position), updated_at = now()
		WHERE e.key = $1
		RETURNING e.id, e.key, e.names, e.position, rfc3339(e.created_at) AS created_at,
			rfc3339(e.updated_at) AS updated_at`, key, names, c.Position)
	if err != nil {
		return nil, dbErr(err)
	}
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[environmentRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	e := r.environment()
	return &e, nil
}

func (s *EnvironmentStore) Delete(ctx context.Context, key string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM environments
		WHERE key = $1`, key)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
