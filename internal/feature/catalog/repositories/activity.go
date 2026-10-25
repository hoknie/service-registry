package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type ActivityRepository struct{ db *pgtx.DB }

func NewActivityRepository(db *pgtx.DB) *ActivityRepository { return &ActivityRepository{db: db} }

func (s *ActivityRepository) Containers(ctx context.Context, v catalogtypes.Viewer, all bool, projects, containers []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out := map[uuid.UUID][]uuid.UUID{}
	if len(projects) == 0 || len(containers) == 0 {
		return out, nil
	}
	found, err := postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowToStructByName[containedRow], `
		WITH RECURSIVE bound AS (
			SELECT node_id
			FROM viewer_bindings($1, $5::uuid[], $6::boolean)
		),
		up AS (
			SELECT n.id AS project_id, n.id AS node_id, n.parent_id, 0 AS depth
			FROM nodes n
			WHERE n.id = ANY($3)
				AND n.kind = 'project'
			UNION ALL
			SELECT u.project_id, p.id, p.parent_id, u.depth + 1
			FROM up u
			JOIN nodes p ON p.id = u.parent_id
			WHERE u.depth < 64
		),
		readable AS (
			SELECT DISTINCT project_id
			FROM up
			WHERE $2 OR node_id IN (SELECT node_id FROM bound)
		)
		SELECT DISTINCT u.node_id AS container_id, u.project_id
		FROM up u
		JOIN readable r ON r.project_id = u.project_id
		WHERE u.depth > 0
			AND u.node_id = ANY($4)`, v.UserID, all, projects, containers, v.Nodes, v.Admin)
	if err != nil {
		return nil, err
	}
	for _, r := range found {
		out[r.ContainerID] = append(out[r.ContainerID], r.ProjectID)
	}
	return out, nil
}

func (s *ActivityRepository) MetricCounts(ctx context.Context, staleDays uint32) (map[string]int64, error) {
	return postgres.Pairs[string, int64](ctx, s.db.From(ctx), constraints, `
		SELECT 'nodes:' || kind, count(*)
		FROM nodes
		GROUP BY kind
		UNION ALL
		SELECT 'branches:' || CASE
				WHEN gone_at IS NOT NULL THEN 'gone'
				WHEN last_activity_at < now() - make_interval(days => $1::int) THEN 'stale'
				ELSE 'active'
			END, count(*)
		FROM branches
		GROUP BY 1
		UNION ALL
		SELECT 'credentials:project_key', count(*)
		FROM project_keys
		WHERE revoked_at IS NULL
			AND (expires_at IS NULL OR expires_at > now())`, int(staleDays))
}

func (s *ActivityRepository) ProjectPaths(ctx context.Context) ([][]string, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[[]string], `
		WITH RECURSIVE tree AS (
			SELECT id, kind, id AS organization_id, slug::text AS path
			FROM nodes
			WHERE parent_id IS NULL
			UNION ALL
			SELECT n.id, n.kind, t.organization_id, t.path || '/' || n.slug
			FROM nodes n
			JOIN tree t ON n.parent_id = t.id
		)
		SELECT ARRAY[id::text, organization_id::text, path]
		FROM tree
		WHERE kind = 'project'
		ORDER BY path`)
}
