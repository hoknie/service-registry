package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/catalog"
)

func (s *Connections) Signals(ctx context.Context, projects []uuid.UUID, busy bool) (map[uuid.UUID][]catalog.ProcessSignals, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		WITH forge AS (
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
		)
		SELECT a.project_id, a.kind, a.running, a.queued, a.failed, a.code, a.last_at, a.pending
		FROM forge a
		WHERE NOT $2
			OR a.running
			OR a.queued
			OR a.failed
		ORDER BY a.project_id`,
		projects, busy)
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

func (s *Connections) ContainerSignals(ctx context.Context, nodeID uuid.UUID) ([]catalog.ProcessSignals, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT 'forge'::text AS kind,
			bool_or(COALESCE(c.lease_until > now(), false)) AS running,
			bool_or(c.next_run_at <= now() AND NOT COALESCE(c.lease_until > now(), false)) AS queued,
			bool_or(COALESCE(l.status = 'failed', false)) AS failed,
			(array_agg(l.error_code ORDER BY c.created_at, c.id) FILTER (WHERE l.status = 'failed'))[1] AS code,
			rfc3339(max(l.finished_at)) AS last_at
		FROM forge_connections c
		LEFT JOIN LATERAL (
			SELECT w.status, w.error_code, w.finished_at
			FROM forge_sync_runs w
			WHERE w.connection_id = c.id
				AND w.status <> 'running'
			ORDER BY w.started_at DESC, w.id DESC
			LIMIT 1
		) l ON true
		WHERE c.node_id = $1
		HAVING count(*) > 0`, nodeID)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (catalog.ProcessSignals, error) {
		var p catalog.ProcessSignals
		var kind string
		err := r.Scan(&kind, &p.Running, &p.Queued, &p.Failed, &p.Code, &p.LastAt)
		p.Kind = catalog.ProcessKind(kind)
		return p, err
	})
	return out, dbErr(err)
}
