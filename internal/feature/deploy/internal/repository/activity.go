package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/catalog"
)

func (s *Clusters) Signals(ctx context.Context, projects []uuid.UUID, busy bool) (map[uuid.UUID][]catalog.ProcessSignals, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		WITH polled AS (
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
		FROM polled a
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
