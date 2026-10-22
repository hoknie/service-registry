package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type ExclusionRepository struct{ db *pgtx.DB }

func NewExclusionRepository(db *pgtx.DB) *ExclusionRepository {
	return &ExclusionRepository{db: db}
}

func (s *ExclusionRepository) List(ctx context.Context, connID uuid.UUID) ([]forgetypes.Exclusion, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT external_id, full_path, project_id, rfc3339(created_at)
		FROM forge_exclusions
		WHERE connection_id = $1
		ORDER BY created_at DESC, external_id`, connID)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (forgetypes.Exclusion, error) {
		var e forgetypes.Exclusion
		err := r.Scan(&e.ExternalID, &e.FullPath, &e.ProjectID, &e.CreatedAt)
		return e, err
	})
	return out, postgres.Translate(err, constraints)
}

func (s *ExclusionRepository) IDs(ctx context.Context, connID uuid.UUID) (map[string]bool, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT external_id
		FROM forge_exclusions
		WHERE connection_id = $1`, connID)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out := map[string]bool{}
	var id string
	_, err = pgx.ForEachRow(rows, []any{&id}, func() error {
		out[id] = true
		return nil
	})
	return out, postgres.Translate(err, constraints)
}

func (s *ExclusionRepository) Insert(ctx context.Context, u forgetypes.Unlinked) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO forge_exclusions (connection_id, external_id, full_path, project_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (connection_id, external_id)
		DO UPDATE SET full_path = EXCLUDED.full_path, project_id = EXCLUDED.project_id, created_at = now()`,
		u.ConnectionID, u.ExternalID, u.FullPath, u.ProjectID)
	return postgres.Translate(err, constraints)
}

func (s *ExclusionRepository) Delete(ctx context.Context, connID uuid.UUID, externalID string) (*forgetypes.Exclusion, error) {
	var e forgetypes.Exclusion
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
		return nil, postgres.Translate(err, constraints)
	}
	return &e, nil
}
