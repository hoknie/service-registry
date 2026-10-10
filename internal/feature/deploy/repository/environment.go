package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/deploy"
	"svc-registry/internal/platform/postgres"
)

type Environments struct{ db *postgres.DB }

func NewEnvironments(db *postgres.DB) *Environments {
	return &Environments{db: db}
}

func (s *Environments) List(ctx context.Context) ([]deploy.Environment, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
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
	out := make([]deploy.Environment, 0, len(items))
	for _, r := range items {
		out = append(out, r.environment())
	}
	return out, nil
}

func (s *Environments) Insert(ctx context.Context, e deploy.NewEnvironment) (deploy.Environment, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		INSERT INTO environments AS e (id, key, names, position)
		VALUES ($1, $2, $3, $4)
		RETURNING e.id, e.key, e.names, e.position, rfc3339(e.created_at) AS created_at,
			rfc3339(e.updated_at) AS updated_at`, e.ID, e.Key, e.Names, e.Position)
	if err != nil {
		return deploy.Environment{}, dbErr(err)
	}
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[environmentRow])
	if err != nil {
		return deploy.Environment{}, dbErr(err)
	}
	return r.environment(), nil
}

func (s *Environments) Update(ctx context.Context, key string, c deploy.EnvironmentChanges) (*deploy.Environment, error) {
	var names any
	if c.Names != nil {
		names = c.Names
	}
	rows, err := s.db.From(ctx).Query(ctx, `
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

func (s *Environments) Delete(ctx context.Context, key string) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM environments
		WHERE key = $1`, key)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return deploy.ErrNotFound
	}
	return nil
}
