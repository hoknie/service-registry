package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/platform/postgres"
)

type Adoptions struct{ db *postgres.DB }

func NewAdoptions(db *postgres.DB) *Adoptions { return &Adoptions{db: db} }

func (s *Adoptions) Insert(ctx context.Context, connID uuid.UUID, externalID string, projectID uuid.UUID) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO forge_adoptions (connection_id, external_id, project_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (connection_id, external_id)
		DO UPDATE SET project_id = EXCLUDED.project_id, created_at = now()`, connID, externalID, projectID)
	return dbErr(err)
}

func (s *Adoptions) Eligible(ctx context.Context, connID uuid.UUID) (map[string]uuid.UUID, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT a.external_id, a.project_id
		FROM forge_adoptions a
		JOIN forge_connections c ON c.id = a.connection_id
		JOIN nodes n ON n.id = a.project_id AND n.kind = 'project'
		WHERE a.connection_id = $1
			AND NOT EXISTS (SELECT 1 FROM forge_repositories r WHERE r.project_id = a.project_id)
			AND EXISTS (
				WITH RECURSIVE up AS (
					SELECT p.id, p.parent_id
					FROM nodes p
					WHERE p.id = n.parent_id
					UNION ALL
					SELECT q.id, q.parent_id
					FROM nodes q
					JOIN up ON q.id = up.parent_id
				)
				SELECT 1 FROM up WHERE up.id = c.node_id
			)`, connID)
	if err != nil {
		return nil, dbErr(err)
	}
	out := map[string]uuid.UUID{}
	var ext string
	var id uuid.UUID
	_, err = pgx.ForEachRow(rows, []any{&ext, &id}, func() error {
		out[ext] = id
		return nil
	})
	return out, dbErr(err)
}

func (s *Adoptions) Delete(ctx context.Context, connID uuid.UUID, externalID string) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM forge_adoptions
		WHERE connection_id = $1
			AND external_id = $2`, connID, externalID)
	return dbErr(err)
}
