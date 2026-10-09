package catalog

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"svc-registry/internal/postgres"

	domain "svc-registry/internal/catalog"
)

type ActivityStore struct{ pool *pgxpool.Pool }

func NewActivityStore(pool *pgxpool.Pool) *ActivityStore { return &ActivityStore{pool: pool} }

func (s *ActivityStore) Signals(ctx context.Context, projects []uuid.UUID, want domain.ActivityWant) (map[uuid.UUID][]domain.ProcessSignals, error) {
	if len(projects) == 0 {
		return map[uuid.UUID][]domain.ProcessSignals{}, nil
	}
	return signalsOf(ctx, s.pool, projects, want, false)
}

func (s *ActivityStore) Busy(ctx context.Context, want domain.ActivityWant) (map[uuid.UUID][]domain.ProcessSignals, error) {
	return signalsOf(ctx, s.pool, nil, want, true)
}

func (s *ActivityStore) Containers(ctx context.Context, userID uuid.UUID, all bool, projects, containers []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out := map[uuid.UUID][]uuid.UUID{}
	if len(projects) == 0 || len(containers) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		WITH RECURSIVE bound AS (
			SELECT node_id
			FROM role_bindings
			WHERE user_id = $1
			UNION ALL
			SELECT b.node_id
			FROM role_bindings b
			JOIN group_members gm ON gm.group_id = b.group_id
			WHERE gm.user_id = $1
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
			AND u.node_id = ANY($4)`, userID, all, projects, containers)
	if err != nil {
		return nil, dbErr(err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowToStructByName[containedRow])
	if err != nil {
		return nil, dbErr(err)
	}
	for _, r := range found {
		out[r.ContainerID] = append(out[r.ContainerID], r.ProjectID)
	}
	return out, nil
}

func signalsOf(ctx context.Context, db postgres.DB, projects []uuid.UUID, want domain.ActivityWant, busy bool) (map[uuid.UUID][]domain.ProcessSignals, error) {
	rows, err := db.Query(ctx, `
		WITH collect AS (
			SELECT n.id AS project_id, 'collect'::text AS kind,
				COALESCE(s.lease_until > now(), false) AS running,
				COALESCE(s.next_run_at <= now(), true) AND NOT COALESCE(s.lease_until > now(), false) AS queued,
				COALESCE(l.status = 'failed', false) AS failed,
				l.error_code AS code,
				rfc3339(l.finished_at) AS last_at,
				NULL::bigint AS pending
			FROM nodes n
			LEFT JOIN knowledge_settings s ON s.project_id = n.id
			LEFT JOIN LATERAL (
				SELECT k.status, k.error_code, k.finished_at
				FROM knowledge_scans k
				WHERE k.project_id = n.id
					AND k.kind = 'collect'
				ORDER BY k.finished_at DESC, k.id DESC
				LIMIT 1
			) l ON true
			WHERE n.kind = 'project'
				AND ($1::uuid[] IS NULL OR n.id = ANY($1))
				AND (EXISTS (
					SELECT 1
					FROM knowledge_sources x
					WHERE x.project_id = n.id
				) OR EXISTS (
					SELECT 1
					FROM forge_repositories r
					WHERE r.project_id = n.id
						AND r.orphaned_at IS NULL
				))
		),
		indexing AS (
			SELECT i.project_id, 'index'::text AS kind,
				COALESCE(i.lease_until > now(), false) AS running,
				i.next_run_at <= now() AND NOT COALESCE(i.lease_until > now(), false) AS queued,
				i.failure IS NOT NULL AS failed,
				i.failure AS code,
				rfc3339(i.synced_at) AS last_at,
				CASE WHEN $4 AND $3 <> '' THEN (
					SELECT count(*)
					FROM (
						SELECT DISTINCT ON (branch) id
						FROM knowledge_snapshots
						WHERE project_id = i.project_id
							AND status <> 'failed'
						ORDER BY branch, collected_at DESC, id DESC
					) latest
					JOIN knowledge_files f ON f.snapshot_id = latest.id
					WHERE f.sha256 IS NOT NULL
						AND NOT EXISTS (
							SELECT 1
							FROM knowledge_embeddings e
							WHERE e.sha256 = f.sha256
								AND e.model = $3
						)
				) WHEN $4 THEN 0 END AS pending
			FROM knowledge_index_state i
			WHERE $2
				AND ($1::uuid[] IS NULL OR i.project_id = ANY($1))
		),
		forge AS (
			SELECT r.project_id, 'forge'::text AS kind,
				COALESCE(c.lease_until > now(), false) AS running,
				c.next_run_at <= now() AND NOT COALESCE(c.lease_until > now(), false) AS queued,
				COALESCE(l.status = 'failed', false) AS failed,
				l.error_code AS code,
				rfc3339(l.finished_at) AS last_at,
				NULL::bigint AS pending
			FROM forge_repositories r
			JOIN forge_connections c ON c.id = r.connection_id
			LEFT JOIN LATERAL (
				SELECT w.status, w.error_code, w.finished_at
				FROM forge_sync_runs w
				WHERE w.connection_id = c.id
					AND w.status <> 'running'
				ORDER BY w.started_at DESC, w.id DESC
				LIMIT 1
			) l ON true
			WHERE r.orphaned_at IS NULL
				AND ($1::uuid[] IS NULL OR r.project_id = ANY($1))
		),
		polled AS (
			SELECT w.project_id, 'clusters'::text AS kind,
				bool_or(COALESCE(c.lease_until > now(), false)) AS running,
				bool_or(c.next_run_at <= now() AND NOT COALESCE(c.lease_until > now(), false)) AS queued,
				bool_or(c.status = 'error') AS failed,
				(array_agg(c.last_error ->> 'code' ORDER BY c.name) FILTER (WHERE c.status = 'error'))[1] AS code,
				rfc3339(max(c.last_polled_at)) AS last_at,
				NULL::bigint AS pending
			FROM (
				SELECT DISTINCT project_id, cluster_id
				FROM cluster_workloads
				WHERE project_id IS NOT NULL
					AND ($1::uuid[] IS NULL OR project_id = ANY($1))
			) w
			JOIN clusters c ON c.id = w.cluster_id
				AND c.enabled
			GROUP BY w.project_id
		)
		SELECT a.project_id, a.kind, a.running, a.queued, a.failed, a.code, a.last_at, a.pending
		FROM (
			SELECT * FROM collect
			UNION ALL
			SELECT * FROM indexing
			UNION ALL
			SELECT * FROM forge
			UNION ALL
			SELECT * FROM polled
		) a
		WHERE NOT $5
			OR a.running
			OR a.queued
			OR a.failed
		ORDER BY a.project_id,
			CASE a.kind WHEN 'collect' THEN 0 WHEN 'index' THEN 1 WHEN 'forge' THEN 2 ELSE 3 END`,
		projects, want.Indexing, want.Model, want.Pending, busy)
	if err != nil {
		return nil, dbErr(err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowToStructByName[signalRow])
	if err != nil {
		return nil, dbErr(err)
	}
	out := map[uuid.UUID][]domain.ProcessSignals{}
	for _, r := range found {
		out[r.ProjectID] = append(out[r.ProjectID], domain.ProcessSignals{Kind: domain.ProcessKind(r.Kind), Running: r.Running,
			Queued: r.Queued, Failed: r.Failed, Code: r.Code, LastAt: r.LastAt, Pending: r.Pending})
	}
	return out, nil
}
