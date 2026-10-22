package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type CollectionRepository struct{ db *pgtx.DB }

func NewCollectionRepository(db *pgtx.DB) *CollectionRepository {
	return &CollectionRepository{db: db}
}

func (s *CollectionRepository) RequestCollect(ctx context.Context, projectID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO knowledge_settings (project_id, force)
		VALUES ($1, true)
		ON CONFLICT (project_id)
		DO UPDATE SET force = true, next_run_at = now()`, projectID)
	return err
}

func (s *CollectionRepository) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	if _, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO knowledge_settings (project_id)
		SELECT r.project_id
		FROM forge_repositories r
		WHERE r.orphaned_at IS NULL
		UNION
		SELECT project_id
		FROM knowledge_sources
		ON CONFLICT
		DO NOTHING`); err != nil {
		return nil, err
	}
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
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
}

func (s *CollectionRepository) Extend(ctx context.Context, projectID uuid.UUID, leaseSecs int) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_settings
		SET lease_until = now() + make_interval(secs => $2)
		WHERE project_id = $1`, projectID, float64(leaseSecs))
	return err
}

func (s *CollectionRepository) Release(ctx context.Context, projectID uuid.UUID, afterSecs uint32, clearForce bool) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE knowledge_settings
		SET lease_until = NULL, next_run_at = now() + make_interval(secs => $2),
			force = CASE WHEN $3 THEN false ELSE force END
		WHERE project_id = $1`, projectID, float64(afterSecs), clearForce)
	return err
}

func (s *CollectionRepository) Project(ctx context.Context, projectID uuid.UUID) (knowledgetypes.Project, error) {
	p, err := postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (knowledgetypes.Project, error) {
		var p knowledgetypes.Project
		err := r.Scan(&p.ID, &p.DefaultBranch, &p.ConnectionID, &p.Force)
		p.Synced = p.ConnectionID != nil
		return p, err
	}, `
		SELECT n.id, COALESCE(n.default_branch, r.default_branch, '') AS default_branch, r.connection_id,
			COALESCE(s.force, false) AS force
		FROM nodes n
		LEFT JOIN forge_repositories r ON r.project_id = n.id AND r.orphaned_at IS NULL
		LEFT JOIN knowledge_settings s ON s.project_id = n.id
		WHERE n.id = $1
			AND n.kind = 'project'`, projectID)
	if err != nil {
		return knowledgetypes.Project{}, err
	}
	if p == nil {
		return knowledgetypes.Project{}, apperr.ErrNotFound
	}
	return *p, nil
}
