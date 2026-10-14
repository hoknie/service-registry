package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/forge"
	"svc-registry/internal/platform/postgres"
)

type Exclusions struct{ db *postgres.DB }

func NewExclusions(db *postgres.DB) *Exclusions { return &Exclusions{db: db} }

func (s *Exclusions) List(ctx context.Context, connID uuid.UUID) ([]forge.Exclusion, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT external_id, full_path, project_id, rfc3339(created_at)
		FROM forge_exclusions
		WHERE connection_id = $1
		ORDER BY created_at DESC, external_id`, connID)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (forge.Exclusion, error) {
		var e forge.Exclusion
		err := r.Scan(&e.ExternalID, &e.FullPath, &e.ProjectID, &e.CreatedAt)
		return e, err
	})
	return out, dbErr(err)
}

func (s *Exclusions) IDs(ctx context.Context, connID uuid.UUID) (map[string]bool, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT external_id
		FROM forge_exclusions
		WHERE connection_id = $1`, connID)
	if err != nil {
		return nil, dbErr(err)
	}
	out := map[string]bool{}
	var id string
	_, err = pgx.ForEachRow(rows, []any{&id}, func() error {
		out[id] = true
		return nil
	})
	return out, dbErr(err)
}

func (s *Exclusions) Insert(ctx context.Context, u forge.Unlinked) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO forge_exclusions (connection_id, external_id, full_path, project_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (connection_id, external_id)
		DO UPDATE SET full_path = EXCLUDED.full_path, project_id = EXCLUDED.project_id, created_at = now()`,
		u.ConnectionID, u.ExternalID, u.FullPath, u.ProjectID)
	return dbErr(err)
}

func (s *Exclusions) Delete(ctx context.Context, connID uuid.UUID, externalID string) (*forge.Exclusion, error) {
	var e forge.Exclusion
	err := s.db.From(ctx).QueryRow(ctx, `
		DELETE FROM forge_exclusions
		WHERE connection_id = $1
			AND external_id = $2
		RETURNING external_id, full_path, project_id, rfc3339(created_at)`, connID, externalID).
		Scan(&e.ExternalID, &e.FullPath, &e.ProjectID, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &e, nil
}
