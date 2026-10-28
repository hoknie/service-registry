package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/postgres"
)

func (s *BranchRepository) UpsertIngest(ctx context.Context, id, projectID uuid.UUID, name string, head *string, at string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO branches AS b (id, project_id, name, head_sha, sources, last_activity_at)
		VALUES ($1, $2, $3, $4, ARRAY['ingest'], $5::timestamptz)
		ON CONFLICT (project_id, name)
		DO UPDATE SET sources = ARRAY(
			SELECT s
			FROM unnest(b.sources || ARRAY['ingest']::text[]) s
			GROUP BY s
			ORDER BY array_position(ARRAY['repo_sync', 'repository', 'ingest', 'cluster', 'manual']::text[], s)
		),
			head_sha = CASE
				WHEN $4::text IS NOT NULL AND ($5::timestamptz > b.last_activity_at OR b.head_sha IS NULL) THEN $4
				ELSE b.head_sha END,
			last_activity_at = GREATEST(b.last_activity_at, $5::timestamptz), gone_at = NULL, updated_at = now()`,
		id, projectID, name, head, at)
	return err
}

func (s *BranchRepository) UpsertRepoSync(ctx context.Context, id, projectID uuid.UUID, name string, head *string, protected *bool, committed *time.Time) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO branches AS b (id, project_id, name, head_sha, protected, sources, last_activity_at)
		VALUES ($1, $2, $3, $4, $5, ARRAY['repo_sync'], COALESCE($6, now()))
		ON CONFLICT (project_id, name)
		DO UPDATE SET sources = ARRAY(
			SELECT s
			FROM unnest(b.sources || ARRAY['repo_sync']::text[]) s
			GROUP BY s
			ORDER BY array_position(ARRAY['repo_sync', 'repository', 'ingest', 'cluster', 'manual']::text[], s)
		), head_sha = COALESCE($4, b.head_sha), protected = COALESCE($5, b.protected),
			last_activity_at = CASE
				WHEN $6::timestamptz IS NOT NULL THEN GREATEST(b.last_activity_at, $6)
				WHEN $4::text IS DISTINCT FROM b.head_sha THEN now()
				ELSE b.last_activity_at END,
			gone_at = NULL, updated_at = now()`, id, projectID, name, head, protected, committed)
	return err
}

func (s *BranchRepository) UpsertCluster(ctx context.Context, id, projectID uuid.UUID, name string, head *string, activity *time.Time) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO branches AS b (id, project_id, name, head_sha, sources, last_activity_at)
		VALUES ($1, $2, $3, $4, ARRAY['cluster'], COALESCE($5::timestamptz, now()))
		ON CONFLICT (project_id, name)
		DO UPDATE SET sources = ARRAY(
			SELECT s
			FROM unnest(b.sources || ARRAY['cluster']::text[]) s
			GROUP BY s
			ORDER BY array_position(ARRAY['repo_sync', 'repository', 'ingest', 'cluster', 'manual']::text[], s)
		),
			head_sha = CASE
				WHEN $5::timestamptz IS NOT NULL AND $4::text IS NOT NULL THEN $4
				ELSE b.head_sha END,
			last_activity_at = CASE
				WHEN $5::timestamptz IS NOT NULL THEN GREATEST(b.last_activity_at, $5)
				ELSE b.last_activity_at END,
			gone_at = NULL, updated_at = now()`, id, projectID, name, head, activity)
	return err
}

func (s *BranchRepository) SyncRepository(ctx context.Context, projectID uuid.UUID, heads []catalogtypes.BranchHead) error {
	ids := make([]uuid.UUID, 0, len(heads))
	names := make([]string, 0, len(heads))
	shas := make([]*string, 0, len(heads))
	for _, h := range heads {
		ids = append(ids, h.ID)
		names = append(names, h.Name)
		shas = append(shas, h.Head)
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		WITH incoming AS (
			SELECT *
			FROM unnest($2::uuid[], $3::text[], $4::text[]) AS i(id, name, head)
		),
		gone AS (
			UPDATE branches
			SET gone_at = now(), updated_at = now()
			WHERE project_id = $1
				AND 'repository' = ANY(sources)
				AND gone_at IS NULL
				AND name <> ALL($3::text[])
		)
		INSERT INTO branches AS b (id, project_id, name, head_sha, sources, last_activity_at)
		SELECT i.id, $1, i.name, i.head, ARRAY['repository'], now()
		FROM incoming i
		ON CONFLICT (project_id, name)
		DO UPDATE SET sources = ARRAY(
			SELECT s
			FROM unnest(b.sources || ARRAY['repository']::text[]) s
			GROUP BY s
			ORDER BY array_position(ARRAY['repo_sync', 'repository', 'ingest', 'cluster', 'manual']::text[], s)
		), head_sha = COALESCE(EXCLUDED.head_sha, b.head_sha),
			last_activity_at = CASE
				WHEN EXCLUDED.head_sha IS DISTINCT FROM b.head_sha THEN now()
				ELSE b.last_activity_at END,
			gone_at = NULL, updated_at = now()`, projectID, ids, names, shas)
	return err
}

func (s *BranchRepository) MarkRepositoryDefault(ctx context.Context, projectID uuid.UUID, name string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE branches b
		SET is_default = true, updated_at = now()
		WHERE b.project_id = $1
			AND b.name = $2
			AND NOT EXISTS (
				SELECT 1
				FROM nodes n
				WHERE n.id = $1
					AND n.default_branch IS NOT NULL
			)
			AND NOT EXISTS (
				SELECT 1
				FROM branches d
				WHERE d.project_id = $1
					AND d.is_default
			)`, projectID, name)
	return err
}
