package repositories

import (
	"context"

	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type EnvironmentRepository struct{ db *pgtx.DB }

func NewEnvironmentRepository(db *pgtx.DB) *EnvironmentRepository {
	return &EnvironmentRepository{db: db}
}

func (s *EnvironmentRepository) List(ctx context.Context) ([]deploytypes.Environment, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(environmentRow.environment), `
		SELECT e.id, e.key, e.names, e.position, rfc3339(e.created_at) AS created_at,
			rfc3339(e.updated_at) AS updated_at
		FROM environments e
		ORDER BY e.position, e.key`)
}

func (s *EnvironmentRepository) Insert(ctx context.Context, e deploytypes.NewEnvironment) (deploytypes.Environment, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, postgres.Struct(environmentRow.environment), `
		INSERT INTO environments AS e (id, key, names, position)
		VALUES ($1, $2, $3, $4)
		RETURNING e.id, e.key, e.names, e.position, rfc3339(e.created_at) AS created_at,
			rfc3339(e.updated_at) AS updated_at`, e.ID, e.Key, e.Names, e.Position)
}

func (s *EnvironmentRepository) Update(ctx context.Context, key string, c deploytypes.EnvironmentChanges) (*deploytypes.Environment, error) {
	var names any
	if c.Names != nil {
		names = c.Names
	}
	return postgres.Find(ctx, s.db.From(ctx), constraints, postgres.Struct(environmentRow.environment), `
		UPDATE environments AS e
		SET names = COALESCE($2, e.names), position = COALESCE($3, e.position), updated_at = now()
		WHERE e.key = $1
		RETURNING e.id, e.key, e.names, e.position, rfc3339(e.created_at) AS created_at,
			rfc3339(e.updated_at) AS updated_at`, key, names, c.Position)
}

func (s *EnvironmentRepository) Delete(ctx context.Context, key string) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM environments
		WHERE key = $1`, key)
}
