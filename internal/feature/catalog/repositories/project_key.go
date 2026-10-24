package repositories

import (
	"context"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type ProjectKeyRepository struct{ db *pgtx.DB }

func NewProjectKeyRepository(db *pgtx.DB) *ProjectKeyRepository {
	return &ProjectKeyRepository{db: db}
}

func (s *ProjectKeyRepository) List(ctx context.Context, projectID uuid.UUID) ([]catalogtypes.ProjectKey, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, scanKey, `
		SELECT k.id, k.project_id, k.prefix, rfc3339(k.created_at) AS created_at,
			rfc3339(k.last_used_at) AS last_used_at, rfc3339(k.expires_at) AS expires_at,
			rfc3339(k.revoked_at) AS revoked_at,
			CASE
				WHEN k.revoked_at IS NOT NULL THEN 'revoked'
				WHEN k.expires_at <= now() THEN 'expired'
				ELSE 'active' END AS status
		FROM project_keys k
		WHERE k.project_id = $1
		ORDER BY k.created_at DESC, k.id DESC`, projectID)
}

func (s *ProjectKeyRepository) ExpireActive(ctx context.Context, projectID uuid.UUID, graceSecs uint64) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE project_keys
		SET expires_at = LEAST(COALESCE(expires_at, 'infinity'), now() + make_interval(secs => $2))
		WHERE project_id = $1
			AND revoked_at IS NULL
			AND (expires_at IS NULL OR expires_at > now())`, projectID, float64(bigint(graceSecs)))
	return err
}

func (s *ProjectKeyRepository) Insert(ctx context.Context, k catalogtypes.NewKey) (catalogtypes.ProjectKey, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, scanKey, `
		INSERT INTO project_keys AS k (id, project_id, prefix, key_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING k.id, k.project_id, k.prefix, rfc3339(k.created_at) AS created_at,
			rfc3339(k.last_used_at) AS last_used_at, rfc3339(k.expires_at) AS expires_at,
			rfc3339(k.revoked_at) AS revoked_at,
			CASE
				WHEN k.revoked_at IS NOT NULL THEN 'revoked'
				WHEN k.expires_at <= now() THEN 'expired'
				ELSE 'active' END AS status`, k.ID, k.ProjectID, k.Prefix, k.Hash[:])
}

func (s *ProjectKeyRepository) Revoke(ctx context.Context, projectID, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE project_keys
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE project_id = $1
			AND id = $2`, projectID, id)
}

func (s *ProjectKeyRepository) FindActive(ctx context.Context, projectID uuid.UUID, hash [32]byte) (*catalogtypes.ActiveKey, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, postgres.Struct(keyUseRow.activeKey), `
		SELECT id, last_used_at IS NULL OR last_used_at < now() - interval '60 seconds' AS stale
		FROM project_keys
		WHERE key_hash = $1
			AND project_id = $2
			AND revoked_at IS NULL
			AND (expires_at IS NULL OR expires_at > now())`, hash[:], projectID)
}

func (s *ProjectKeyRepository) Touch(ctx context.Context, id uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE project_keys
		SET last_used_at = now()
		WHERE id = $1`, id)
	return err
}
