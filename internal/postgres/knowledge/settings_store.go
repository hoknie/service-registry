package knowledge

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/knowledge"
)

type SettingsStore struct{ pool *pgxpool.Pool }

func NewSettingsStore(pool *pgxpool.Pool) *SettingsStore { return &SettingsStore{pool: pool} }

func (s *SettingsStore) RequestCollect(ctx context.Context, projectID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO knowledge_settings (project_id, force)
		VALUES ($1, true)
		ON CONFLICT (project_id)
		DO UPDATE SET force = true, next_run_at = now()`, projectID)
	return dbErr(err)
}

func (s *SettingsStore) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO knowledge_settings (project_id)
		SELECT r.project_id
		FROM forge_repositories r
		WHERE r.orphaned_at IS NULL
		UNION
		SELECT project_id
		FROM knowledge_sources
		ON CONFLICT
		DO NOTHING`); err != nil {
		return nil, dbErr(err)
	}
	rows, err := s.pool.Query(ctx, `
		UPDATE knowledge_settings s
		SET lease_until = now() + make_interval(secs => $2)
		WHERE s.project_id IN (
			SELECT k.project_id
			FROM knowledge_settings k
			WHERE k.next_run_at <= now()
				AND (k.lease_until IS NULL OR k.lease_until < now())
				AND (EXISTS (
					SELECT 1
					FROM forge_repositories r
					WHERE r.project_id = k.project_id
						AND r.orphaned_at IS NULL
				) OR EXISTS (
					SELECT 1
					FROM knowledge_sources x
					WHERE x.project_id = k.project_id
				))
			ORDER BY k.next_run_at
			LIMIT $1
			FOR UPDATE OF k SKIP LOCKED
		)
		RETURNING s.project_id`, limit, float64(leaseSecs))
	if err != nil {
		return nil, dbErr(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	return ids, dbErr(err)
}

func (s *SettingsStore) Extend(ctx context.Context, projectID uuid.UUID, leaseSecs int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE knowledge_settings
		SET lease_until = now() + make_interval(secs => $2)
		WHERE project_id = $1`, projectID, float64(leaseSecs))
	return dbErr(err)
}

func (s *SettingsStore) Release(ctx context.Context, projectID uuid.UUID, afterSecs uint32, clearForce bool) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE knowledge_settings
		SET lease_until = NULL, next_run_at = now() + make_interval(secs => $2),
			force = CASE WHEN $3 THEN false ELSE force END
		WHERE project_id = $1`, projectID, float64(afterSecs), clearForce)
	return dbErr(err)
}

func (s *SettingsStore) Project(ctx context.Context, projectID uuid.UUID) (domain.Project, error) {
	var p domain.Project
	err := s.pool.QueryRow(ctx, `
		SELECT n.id, COALESCE(n.default_branch, r.default_branch, '') AS default_branch, r.connection_id,
			COALESCE(s.force, false) AS force
		FROM nodes n
		LEFT JOIN forge_repositories r ON r.project_id = n.id AND r.orphaned_at IS NULL
		LEFT JOIN knowledge_settings s ON s.project_id = n.id
		WHERE n.id = $1
			AND n.kind = 'project'`, projectID).Scan(&p.ID, &p.DefaultBranch, &p.ConnectionID, &p.Force)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, domain.ErrNotFound
	}
	p.Synced = p.ConnectionID != nil
	return p, dbErr(err)
}
