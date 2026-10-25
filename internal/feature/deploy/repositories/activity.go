package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/postgres"
)

func (s *ClusterRepository) Signals(ctx context.Context, projects []uuid.UUID, busy bool) (map[uuid.UUID][]catalogtypes.ProcessSignals, error) {
	found, err := postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowToStructByPos[catalogtypes.ProjectSignals], `
		WITH polled AS (
			SELECT w.project_id, 'clusters'::text AS kind,
				bool_or(COALESCE(c.lease_until > now(), false)) AS running,
				bool_or(c.mode = 'poll' AND c.next_run_at <= now() AND NOT COALESCE(c.lease_until > now(), false)) AS queued,
				bool_or(c.status = 'error' OR q.silent) AS failed,
				(array_agg(CASE WHEN q.silent THEN 'k8s.agent_silent' ELSE c.last_error ->> 'code' END ORDER BY c.name)
					FILTER (WHERE c.status = 'error' OR q.silent))[1] AS code,
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
			CROSS JOIN LATERAL (
				SELECT c.mode = 'agent'
					AND COALESCE(c.agent_reported_at, c.updated_at) < now() - make_interval(secs => 3 * GREATEST(c.interval_secs,
						agent_report_gap(c.interval_secs, c.agent_workloads, $3))) AS silent
			) q
			GROUP BY w.project_id
		)
		SELECT a.project_id, a.kind, a.running, a.queued, a.failed, a.code, a.last_at, a.pending
		FROM polled a
		WHERE NOT $2
			OR a.running
			OR a.queued
			OR a.failed
		ORDER BY a.project_id`,
		projects, busy, s.perGap)
	if err != nil {
		return nil, err
	}
	return catalogtypes.SignalsByProject(found), nil
}

func (s *ClusterRepository) MetricCounts(ctx context.Context) (map[string]int64, error) {
	return postgres.Pairs[string, int64](ctx, s.db.From(ctx), constraints, `
		SELECT 'clusters:' || c.mode || '/' || CASE
				WHEN c.mode = 'agent' AND COALESCE(c.agent_reported_at, c.updated_at) < now() - make_interval(secs => 3 * GREATEST(
					c.interval_secs, agent_report_gap(c.interval_secs, c.agent_workloads, $1))) THEN 'silent'
				WHEN c.status = 'error' THEN 'failed'
				ELSE 'ok'
			END, count(*)
		FROM clusters c
		GROUP BY 1
		UNION ALL
		SELECT 'workloads', count(*)
		FROM cluster_workloads
		WHERE gone_at IS NULL
		UNION ALL
		SELECT 'credentials:agent_token', count(*)
		FROM cluster_agent_tokens
		WHERE revoked_at IS NULL
			AND (expires_at IS NULL OR expires_at > now())
		UNION ALL
		SELECT 'drifts', count(*)
		FROM service_environments e
		JOIN service_deployments d ON d.id = e.deployment_id
		WHERE EXISTS (
			SELECT 1
			FROM cluster_workloads w
			WHERE w.project_id = e.project_id
				AND w.service = e.service
				AND w.environment = e.environment
				AND w.gone_at IS NULL
				AND w.version IS NOT NULL
				AND w.version <> d.version
		)`, s.perGap)
}

func (s *ClusterRepository) QueueCounts(ctx context.Context) (map[string]int64, error) {
	return postgres.Pairs[string, int64](ctx, s.db.From(ctx), constraints, `
		SELECT 'queue:cluster_polls/' || CASE WHEN lease_until > now() THEN 'leased' ELSE 'due' END, count(*)
		FROM clusters
		WHERE enabled
			AND mode = 'poll'
			AND (lease_until > now() OR next_run_at <= now())
		GROUP BY 1`)
}
