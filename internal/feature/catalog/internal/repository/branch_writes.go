package repository

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/platform/postgres"
)

var hexSHA = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

func branchSHA(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.ToLower(*s)
	if !hexSHA.MatchString(v) {
		return nil
	}
	return &v
}

func UpsertIngestBranch(ctx context.Context, db postgres.Querier, projectID uuid.UUID, name string, commit *string, at string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO branches AS b (id, project_id, name, head_sha, sources, last_activity_at)
		VALUES ($1, $2, $3, $4, ARRAY['ingest'], $5::timestamptz)
		ON CONFLICT (project_id, name)
		DO UPDATE SET sources = ARRAY(
			SELECT s
			FROM unnest(b.sources || ARRAY['ingest']::text[]) s
			GROUP BY s
			ORDER BY array_position(ARRAY['forge', 'repository', 'ingest', 'cluster', 'manual']::text[], s)
		),
			head_sha = CASE
				WHEN $4::text IS NOT NULL AND ($5::timestamptz > b.last_activity_at OR b.head_sha IS NULL) THEN $4
				ELSE b.head_sha END,
			last_activity_at = GREATEST(b.last_activity_at, $5::timestamptz), gone_at = NULL, updated_at = now()`,
		uuid.Must(uuid.NewV7()), projectID, name, branchSHA(commit), at)
	return err
}

func UpsertForgeBranch(ctx context.Context, db postgres.Querier, projectID uuid.UUID, name, head string, protected *bool, committed *time.Time) error {
	_, err := db.Exec(ctx, `
		INSERT INTO branches AS b (id, project_id, name, head_sha, protected, sources, last_activity_at)
		VALUES ($1, $2, $3, $4, $5, ARRAY['forge'], COALESCE($6, now()))
		ON CONFLICT (project_id, name)
		DO UPDATE SET sources = ARRAY(
			SELECT s
			FROM unnest(b.sources || ARRAY['forge']::text[]) s
			GROUP BY s
			ORDER BY array_position(ARRAY['forge', 'repository', 'ingest', 'cluster', 'manual']::text[], s)
		), head_sha = COALESCE($4, b.head_sha), protected = COALESCE($5, b.protected),
			last_activity_at = CASE
				WHEN $6::timestamptz IS NOT NULL THEN GREATEST(b.last_activity_at, $6)
				WHEN $4::text IS DISTINCT FROM b.head_sha THEN now()
				ELSE b.last_activity_at END,
			gone_at = NULL, updated_at = now()`, uuid.Must(uuid.NewV7()), projectID, name, branchSHA(&head), protected, committed)
	return err
}

func UpsertClusterBranch(ctx context.Context, db postgres.Querier, projectID uuid.UUID, name string, commit *string, activity *time.Time) error {
	_, err := db.Exec(ctx, `
		INSERT INTO branches AS b (id, project_id, name, head_sha, sources, last_activity_at)
		VALUES ($1, $2, $3, $4, ARRAY['cluster'], COALESCE($5::timestamptz, now()))
		ON CONFLICT (project_id, name)
		DO UPDATE SET sources = ARRAY(
			SELECT s
			FROM unnest(b.sources || ARRAY['cluster']::text[]) s
			GROUP BY s
			ORDER BY array_position(ARRAY['forge', 'repository', 'ingest', 'cluster', 'manual']::text[], s)
		),
			head_sha = CASE
				WHEN $5::timestamptz IS NOT NULL AND $4::text IS NOT NULL THEN $4
				ELSE b.head_sha END,
			last_activity_at = CASE
				WHEN $5::timestamptz IS NOT NULL THEN GREATEST(b.last_activity_at, $5)
				ELSE b.last_activity_at END,
			gone_at = NULL, updated_at = now()`, uuid.Must(uuid.NewV7()), projectID, name, branchSHA(commit), activity)
	return err
}

func SyncRepositoryBranches(ctx context.Context, db postgres.Querier, projectID uuid.UUID, heads map[string]string, defaultBranch string) error {
	ids := make([]uuid.UUID, 0, len(heads))
	names := make([]string, 0, len(heads))
	shas := make([]*string, 0, len(heads))
	for name, head := range heads {
		ids = append(ids, uuid.Must(uuid.NewV7()))
		names = append(names, name)
		shas = append(shas, branchSHA(&head))
	}
	if _, err := db.Exec(ctx, `
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
			ORDER BY array_position(ARRAY['forge', 'repository', 'ingest', 'cluster', 'manual']::text[], s)
		), head_sha = COALESCE(EXCLUDED.head_sha, b.head_sha),
			last_activity_at = CASE
				WHEN EXCLUDED.head_sha IS DISTINCT FROM b.head_sha THEN now()
				ELSE b.last_activity_at END,
			gone_at = NULL, updated_at = now()`, projectID, ids, names, shas); err != nil {
		return err
	}
	if defaultBranch == "" {
		return nil
	}
	_, err := db.Exec(ctx, `
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
			)`, projectID, defaultBranch)
	return err
}
