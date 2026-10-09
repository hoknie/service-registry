package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/catalog"
	"svc-registry/internal/postgres"
)

type ProjectKeyStore struct{ pool *pgxpool.Pool }

func NewProjectKeyStore(pool *pgxpool.Pool) *ProjectKeyStore { return &ProjectKeyStore{pool: pool} }

var keyColumns = "k.id, k.project_id, k.prefix, " +
	postgres.RFC3339("k.created_at") + " AS created_at, " +
	postgres.RFC3339("k.last_used_at") + " AS last_used_at, " +
	postgres.RFC3339("k.expires_at") + " AS expires_at, " +
	postgres.RFC3339("k.revoked_at") + " AS revoked_at, CASE WHEN k.revoked_at IS NOT NULL THEN 'revoked' " +
	"WHEN k.expires_at <= now() THEN 'expired' ELSE 'active' END AS status"

const keyUsable = "revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())"

var (
	keyList         = "SELECT " + keyColumns + " FROM project_keys k WHERE k.project_id = $1 ORDER BY k.created_at DESC, k.id DESC"
	keyExpireOthers = "UPDATE project_keys SET expires_at = LEAST(COALESCE(expires_at, 'infinity'), " +
		"now() + make_interval(secs => $2)) WHERE project_id = $1 AND " + keyUsable
	keyInsert = "INSERT INTO project_keys AS k (id, project_id, prefix, key_hash) VALUES ($1, $2, $3, $4) " +
		"RETURNING " + keyColumns
	keyRevoke = "UPDATE project_keys SET revoked_at = COALESCE(revoked_at, now()) " +
		"WHERE project_id = $1 AND id = $2"
	keyVerify = "SELECT id, last_used_at IS NULL OR last_used_at < now() - interval '60 seconds' AS stale " +
		"FROM project_keys WHERE key_hash = $1 AND project_id = $2 AND " + keyUsable
	keyTouch = "UPDATE project_keys SET last_used_at = now() WHERE id = $1"
)

func scanKey(row pgx.Row) (domain.ProjectKey, error) {
	var k domain.ProjectKey
	var status string
	if err := row.Scan(&k.ID, &k.ProjectID, &k.Prefix, &k.CreatedAt, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt, &status); err != nil {
		return domain.ProjectKey{}, err
	}
	st, ok := domain.ParseKeyStatus(status)
	if !ok {
		return domain.ProjectKey{}, internal("key status", status)
	}
	k.Status = st
	return k, nil
}

func (s *ProjectKeyStore) List(ctx context.Context, projectID uuid.UUID) ([]domain.ProjectKey, error) {
	rows, err := s.pool.Query(ctx, keyList, projectID)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.ProjectKey, error) { return scanKey(r) })
	if err != nil {
		return nil, dbErr(err)
	}
	return out, nil
}

func (s *ProjectKeyStore) Rotate(ctx context.Context, k domain.NewKey, graceSecs uint64) (domain.ProjectKey, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.ProjectKey{}, dbErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, keyExpireOthers, k.ProjectID, float64(bigint(graceSecs))); err != nil {
		return domain.ProjectKey{}, dbErr(err)
	}
	created, err := scanKey(tx.QueryRow(ctx, keyInsert, k.ID, k.ProjectID, k.Prefix, k.Hash[:]))
	if err != nil {
		return domain.ProjectKey{}, dbErr(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ProjectKey{}, dbErr(err)
	}
	return created, nil
}

func (s *ProjectKeyStore) Revoke(ctx context.Context, projectID, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, keyRevoke, projectID, id)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *ProjectKeyStore) Verify(ctx context.Context, projectID uuid.UUID, hash [32]byte) (*uuid.UUID, error) {
	var id uuid.UUID
	var stale bool
	err := s.pool.QueryRow(ctx, keyVerify, hash[:], projectID).Scan(&id, &stale)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	if stale {
		if _, err := s.pool.Exec(ctx, keyTouch, id); err != nil {
			return nil, dbErr(err)
		}
	}
	return &id, nil
}
