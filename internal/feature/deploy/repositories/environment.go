package repositories

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

type EnvironmentRepository struct{ db *postgres.DB }

func NewEnvironmentRepository(db *postgres.DB) *EnvironmentRepository {
	return &EnvironmentRepository{db: db}
}

func (s *EnvironmentRepository) List(ctx context.Context) ([]deploytypes.Environment, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT e.id, e.key, e.names, e.position, rfc3339(e.created_at) AS created_at,
			rfc3339(e.updated_at) AS updated_at
		FROM environments e
		ORDER BY e.position, e.key`)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[environmentRow])
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out := make([]deploytypes.Environment, 0, len(items))
	for _, r := range items {
		out = append(out, r.environment())
	}
	return out, nil
}

func (s *EnvironmentRepository) Insert(ctx context.Context, e deploytypes.NewEnvironment) (deploytypes.Environment, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		INSERT INTO environments AS e (id, key, names, position)
		VALUES ($1, $2, $3, $4)
		RETURNING e.id, e.key, e.names, e.position, rfc3339(e.created_at) AS created_at,
			rfc3339(e.updated_at) AS updated_at`, e.ID, e.Key, e.Names, e.Position)
	if err != nil {
		return deploytypes.Environment{}, postgres.Translate(err, constraints)
	}
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[environmentRow])
	if err != nil {
		return deploytypes.Environment{}, postgres.Translate(err, constraints)
	}
	return r.environment(), nil
}

func (s *EnvironmentRepository) Update(ctx context.Context, key string, c deploytypes.EnvironmentChanges) (*deploytypes.Environment, error) {
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
		return nil, postgres.Translate(err, constraints)
	}
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[environmentRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	e := r.environment()
	return &e, nil
}

func (s *EnvironmentRepository) Delete(ctx context.Context, key string) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM environments
		WHERE key = $1`, key)
	if err != nil {
		return postgres.Translate(err, constraints)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}
	return nil
}
