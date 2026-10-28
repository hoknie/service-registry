package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type SpaceRepository struct{ db *pgtx.DB }

func NewSpaceRepository(db *pgtx.DB) *SpaceRepository { return &SpaceRepository{db: db} }

func (s *SpaceRepository) Description(ctx context.Context, organization uuid.UUID) (string, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[string], `
		SELECT description
		FROM skill_spaces
		WHERE organization_id = $1`, organization)
	if err != nil || found == nil {
		return "", err
	}
	return *found, nil
}

func (s *SpaceRepository) SetDescription(ctx context.Context, organization uuid.UUID, description string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO skill_spaces (organization_id, description)
		VALUES ($1, $2)
		ON CONFLICT (organization_id)
		DO UPDATE SET description = $2, updated_at = now()`, organization, description)
	return err
}
