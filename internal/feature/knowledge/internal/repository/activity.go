package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/catalog"
)

func (s *Collections) Signals(ctx context.Context, projects []uuid.UUID, indexing bool, model string, pending, busy bool) (map[uuid.UUID][]catalog.ProcessSignals, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
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
		)
		SELECT a.project_id, a.kind, a.running, a.queued, a.failed, a.code, a.last_at, a.pending
		FROM (
			SELECT * FROM collect
			UNION ALL
			SELECT * FROM indexing
		) a
		WHERE NOT $5
			OR a.running
			OR a.queued
			OR a.failed
		ORDER BY a.project_id, CASE a.kind WHEN 'collect' THEN 0 ELSE 1 END`,
		projects, indexing, model, pending, busy)
	if err != nil {
		return nil, dbErr(err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowToStructByName[signalRow])
	if err != nil {
		return nil, dbErr(err)
	}
	out := map[uuid.UUID][]catalog.ProcessSignals{}
	for _, r := range found {
		out[r.ProjectID] = append(out[r.ProjectID], catalog.ProcessSignals{Kind: catalog.ProcessKind(r.Kind), Running: r.Running,
			Queued: r.Queued, Failed: r.Failed, Code: r.Code, LastAt: r.LastAt, Pending: r.Pending})
	}
	return out, nil
}
