package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type ProjectKeyRepository struct{ db *pgtx.DB }

func NewProjectKeyRepository(db *pgtx.DB) *ProjectKeyRepository {
	return &ProjectKeyRepository{db: db}
}

func scanKey(row pgx.Row) (catalogtypes.ProjectKey, error) {
	var k catalogtypes.ProjectKey
	var status string
	if err := row.Scan(&k.ID, &k.ProjectID, &k.Prefix, &k.CreatedAt, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt, &status); err != nil {
		return catalogtypes.ProjectKey{}, err
	}
	st, ok := catalogtypes.ParseKeyStatus(status)
	if !ok {
		return catalogtypes.ProjectKey{}, apperr.UnknownValue("key status", status)
	}
	k.Status = st
	return k, nil
}

func (s *ProjectKeyRepository) List(ctx context.Context, projectID uuid.UUID) ([]catalogtypes.ProjectKey, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
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
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (catalogtypes.ProjectKey, error) { return scanKey(r) })
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	return out, nil
}

func (s *ProjectKeyRepository) Rotate(ctx context.Context, k catalogtypes.NewKey, graceSecs uint64) (catalogtypes.ProjectKey, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return catalogtypes.ProjectKey{}, postgres.Translate(err, constraints)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		UPDATE project_keys
		SET expires_at = LEAST(COALESCE(expires_at, 'infinity'), now() + make_interval(secs => $2))
		WHERE project_id = $1
			AND revoked_at IS NULL
			AND (expires_at IS NULL OR expires_at > now())`, k.ProjectID, float64(bigint(graceSecs))); err != nil {
		return catalogtypes.ProjectKey{}, postgres.Translate(err, constraints)
	}
	created, err := scanKey(tx.QueryRow(ctx, `
		INSERT INTO project_keys AS k (id, project_id, prefix, key_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING k.id, k.project_id, k.prefix, rfc3339(k.created_at) AS created_at,
			rfc3339(k.last_used_at) AS last_used_at, rfc3339(k.expires_at) AS expires_at,
			rfc3339(k.revoked_at) AS revoked_at,
			CASE
				WHEN k.revoked_at IS NOT NULL THEN 'revoked'
				WHEN k.expires_at <= now() THEN 'expired'
				ELSE 'active' END AS status`, k.ID, k.ProjectID, k.Prefix, k.Hash[:]))
	if err != nil {
		return catalogtypes.ProjectKey{}, postgres.Translate(err, constraints)
	}
	if err := tx.Commit(ctx); err != nil {
		return catalogtypes.ProjectKey{}, postgres.Translate(err, constraints)
	}
	return created, nil
}

func (s *ProjectKeyRepository) Revoke(ctx context.Context, projectID, id uuid.UUID) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		UPDATE project_keys
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE project_id = $1
			AND id = $2`, projectID, id)
	if err != nil {
		return postgres.Translate(err, constraints)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

func (s *ProjectKeyRepository) Verify(ctx context.Context, projectID uuid.UUID, hash [32]byte) (*uuid.UUID, error) {
	var id uuid.UUID
	var stale bool
	err := s.db.From(ctx).QueryRow(ctx, `
		SELECT id, last_used_at IS NULL OR last_used_at < now() - interval '60 seconds' AS stale
		FROM project_keys
		WHERE key_hash = $1
			AND project_id = $2
			AND revoked_at IS NULL
			AND (expires_at IS NULL OR expires_at > now())`, hash[:], projectID).Scan(&id, &stale)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	if stale {
		if _, err := s.db.From(ctx).Exec(ctx, `
			UPDATE project_keys
			SET last_used_at = now()
			WHERE id = $1`, id); err != nil {
			return nil, postgres.Translate(err, constraints)
		}
	}
	return &id, nil
}
