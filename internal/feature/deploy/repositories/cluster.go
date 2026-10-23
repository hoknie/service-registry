package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type ClusterRepository struct {
	db     *pgtx.DB
	perGap int32
}

func NewClusterRepository(db *pgtx.DB, workloadsPerGapSec int32) *ClusterRepository {
	return &ClusterRepository{db: db, perGap: workloadsPerGapSec}
}

func (s *ClusterRepository) List(ctx context.Context) ([]deploytypes.Cluster, error) {
	return clusters(ctx, s.db.From(ctx), nil, s.perGap)
}

func (s *ClusterRepository) Get(ctx context.Context, id uuid.UUID) (*deploytypes.Cluster, error) {
	items, err := clusters(ctx, s.db.From(ctx), &id, s.perGap)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

func clusters(ctx context.Context, q pgtx.Querier, id *uuid.UUID, perGap int32) ([]deploytypes.Cluster, error) {
	return postgres.List(ctx, q, constraints, postgres.Struct(clusterRow.cluster), `
		SELECT c.id, c.mode, c.name, c.environment, c.in_cluster, c.api_url, c.ca_pem, c.credentials_secret_id,
			c.namespaces, c.rules, c.interval_secs, c.enabled,
			CASE WHEN q.silent THEN 'error' ELSE c.status END AS status,
			CASE WHEN q.silent THEN jsonb_build_object('code', 'k8s.agent_silent',
				'message', 'the agent has not reported for three intervals') ELSE c.last_error END AS last_error,
			rfc3339(c.last_polled_at) AS last_polled_at, rfc3339(c.next_run_at) AS next_run_at,
			c.agent_version, rfc3339(c.agent_reported_at) AS agent_reported_at, c.agent_report,
			agent_report_gap(c.interval_secs, c.agent_workloads, $2) AS report_gap_secs, c.agent_workloads, ARRAY(
				SELECT t.prefix
				FROM cluster_agent_tokens t
				WHERE t.cluster_id = c.id
					AND t.revoked_at IS NULL
					AND (t.expires_at IS NULL OR t.expires_at > now())
				ORDER BY t.created_at, t.id
			) AS token_prefixes, (
			SELECT count(*)
			FROM cluster_workloads w
			WHERE w.cluster_id = c.id
				AND w.project_id IS NOT NULL
				AND w.gone_at IS NULL
		) AS workloads, (
			SELECT count(*)
			FROM cluster_workloads w
			WHERE w.cluster_id = c.id
				AND w.project_id IS NULL
				AND w.gone_at IS NULL
		) AS unmatched, rfc3339(c.created_at) AS created_at, rfc3339(c.updated_at) AS updated_at
		FROM clusters c
		CROSS JOIN LATERAL (
			SELECT c.mode = 'agent' AND c.enabled
				AND COALESCE(c.agent_reported_at, c.updated_at) < now() - make_interval(secs => 3 * GREATEST(c.interval_secs,
					agent_report_gap(c.interval_secs, c.agent_workloads, $2))) AS silent
		) q
		WHERE $1::uuid IS NULL
			OR c.id = $1
		ORDER BY lower(c.name), c.id`, id, perGap)
}

func rulesParam(rules []deploytypes.Rule) []deploytypes.Rule {
	if rules == nil {
		return []deploytypes.Rule{}
	}
	return rules
}

func namespacesParam(ns []string) []string {
	if ns == nil {
		return []string{}
	}
	return ns
}

func (s *ClusterRepository) Insert(ctx context.Context, c deploytypes.NewCluster) (deploytypes.Cluster, error) {
	st := c.Settings
	if _, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO clusters (id, mode, name, environment, in_cluster, api_url, ca_pem, credentials_secret_id,
			namespaces, rules, interval_secs, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12)`,
		c.ID, st.Mode, st.Name, st.Environment, st.InCluster, st.APIURL, st.CAPEM,
		c.SecretID, namespacesParam(st.Namespaces), rulesParam(st.Rules),
		st.IntervalSecs, st.Enabled); err != nil {
		return deploytypes.Cluster{}, err
	}
	got, err := s.Get(ctx, c.ID)
	if err != nil {
		return deploytypes.Cluster{}, err
	}
	if got == nil {
		return deploytypes.Cluster{}, apperr.InternalDetail("inserted cluster vanished")
	}
	return *got, nil
}

func (s *ClusterRepository) Update(ctx context.Context, id uuid.UUID, u deploytypes.ClusterUpdate) (*deploytypes.Cluster, error) {
	st := u.Settings
	n, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE clusters
		SET name = $2, environment = $3, in_cluster = $4, api_url = $5, ca_pem = $6, namespaces = $7,
			rules = $8::jsonb, interval_secs = $9, enabled = $10,
			credentials_secret_id = CASE WHEN $4 OR $13 = 'agent' THEN NULL WHEN $11 THEN $12 ELSE credentials_secret_id END,
			next_run_at = CASE WHEN mode <> $13 THEN now() ELSE next_run_at END,
			status = CASE WHEN mode <> $13 THEN 'never' ELSE status END,
			last_error = CASE WHEN mode <> $13 THEN NULL ELSE last_error END,
			agent_version = CASE WHEN $13 = 'poll' THEN NULL ELSE agent_version END,
			agent_reported_at = CASE WHEN $13 = 'poll' THEN NULL ELSE agent_reported_at END,
			agent_report = CASE WHEN $13 = 'poll' THEN NULL ELSE agent_report END,
			agent_workloads = CASE WHEN $13 = 'poll' THEN 0 ELSE agent_workloads END,
			mode = $13,
			updated_at = now()
		WHERE id = $1`, id, st.Name, st.Environment, st.InCluster, st.APIURL, st.CAPEM,
		namespacesParam(st.Namespaces), rulesParam(st.Rules), st.IntervalSecs, st.Enabled,
		u.SecretID != nil, u.SecretID, st.Mode)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	return s.Get(ctx, id)
}

func (s *ClusterRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM clusters
		WHERE id = $1`, id)
}

func (s *ClusterRepository) Stored(ctx context.Context, id uuid.UUID) (*deploytypes.StoredCredentials, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (deploytypes.StoredCredentials, error) {
		var c deploytypes.StoredCredentials
		err := r.Scan(&c.SecretID)
		return c, err
	}, `
		SELECT credentials_secret_id
		FROM clusters
		WHERE id = $1`, id)
}

func (s *ClusterRepository) ScheduleNow(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE clusters
		SET next_run_at = now()
		WHERE id = $1`, id)
}

func (s *ClusterRepository) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		UPDATE clusters
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id IN (
			SELECT id
			FROM clusters
			WHERE enabled
				AND mode = 'poll'
				AND next_run_at <= now()
				AND (lease_until IS NULL OR lease_until < now())
			ORDER BY next_run_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		RETURNING id`, limit, leaseSecs)
}

func (s *ClusterRepository) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE clusters
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id = $1`, id, leaseSecs)
	return err
}

func (s *ClusterRepository) Finish(ctx context.Context, id uuid.UUID, failure *deploytypes.Failure) error {
	status := string(deploytypes.StatusOK)
	var lastError any
	if failure != nil {
		status = string(deploytypes.StatusError)
		lastError = errorJSON{Code: string(failure.Code), Message: failure.Message}
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE clusters
		SET status = $2, last_error = $3::jsonb, last_polled_at = now(),
			next_run_at = now() + make_interval(secs => interval_secs), lease_until = NULL
		WHERE id = $1`, id, status, lastError)
	return err
}

func (s *ClusterRepository) Legacy(ctx context.Context) ([]deploytypes.LegacyCluster, error) {
	tx := s.db.From(ctx)
	if ok, err := postgres.HasColumn(ctx, tx, deploytypes.Table, deploytypes.ColumnCredentials); err != nil || !ok {
		return nil, err
	}
	return postgres.List(ctx, tx, constraints, func(r pgx.CollectableRow) (deploytypes.LegacyCluster, error) {
		var c deploytypes.LegacyCluster
		err := r.Scan(&c.ID, &c.Name, &c.Enc, &c.Ref)
		return c, err
	}, `
		SELECT id, name, credentials_enc, credentials_ref
		FROM clusters
		WHERE credentials_secret_id IS NULL
			AND (credentials_enc IS NOT NULL OR credentials_ref IS NOT NULL)
		ORDER BY id`)
}

func (s *ClusterRepository) AdoptSecret(ctx context.Context, id, secret uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE clusters
		SET credentials_secret_id = $2, credentials_enc = NULL, credentials_ref = NULL, credentials_fingerprint = NULL,
			updated_at = now()
		WHERE id = $1`, id, secret)
}

func (s *ClusterRepository) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return postgres.Pairs[uuid.UUID, int64](ctx, s.db.From(ctx), constraints, `
		SELECT credentials_secret_id, count(*)
		FROM clusters
		WHERE credentials_secret_id = ANY($1)
		GROUP BY credentials_secret_id`, secrets)
}

func (s *ClusterRepository) LeaseReport(ctx context.Context, id uuid.UUID, leaseSecs int) (deploytypes.ReportLease, error) {
	got, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		UPDATE clusters
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id = $1
			AND mode = 'agent'
			AND enabled
			AND (lease_until IS NULL OR lease_until < now())
			AND (agent_reported_at IS NULL
				OR agent_reported_at <= now() - make_interval(secs => agent_report_gap(interval_secs, agent_workloads, $3)))
		RETURNING id`, id, leaseSecs, s.perGap)
	if err != nil || got != nil {
		return deploytypes.ReportLease{Leased: got != nil}, err
	}
	lease, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowToStructByPos[deploytypes.ReportLease], `
		SELECT false, NOT enabled, COALESCE(lease_until > now(), false),
			GREATEST(1, ceil(extract(epoch FROM agent_reported_at
				+ make_interval(secs => agent_report_gap(interval_secs, agent_workloads, $2)) - now())))::bigint
		FROM clusters
		WHERE id = $1`, id, s.perGap)
	if err != nil || lease == nil {
		return deploytypes.ReportLease{Disabled: true}, err
	}
	return *lease, nil
}

func (s *ClusterRepository) FinishReport(ctx context.Context, id uuid.UUID, version string, workloads int,
	summary deploytypes.ReportSummary) (int32, error) {
	status := string(deploytypes.StatusOK)
	var lastError any
	if summary.Error != nil {
		status = string(deploytypes.StatusError)
		lastError = errorJSON{Code: summary.Error.Code, Message: summary.Error.Message}
	}
	gap, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[int32], `
		UPDATE clusters
		SET status = $2, last_error = $3::jsonb, last_polled_at = now(), agent_reported_at = now(),
			agent_version = $4, agent_report = $5::jsonb, agent_workloads = $6,
			next_run_at = now() + make_interval(secs => GREATEST(interval_secs, agent_report_gap(interval_secs, $6, $7))),
			lease_until = NULL
		WHERE id = $1
		RETURNING agent_report_gap(interval_secs, agent_workloads, $7)`, id, status, lastError, version, summary, workloads, s.perGap)
	if err != nil || gap == nil {
		return 0, err
	}
	return *gap, nil
}

func (s *ClusterRepository) Release(ctx context.Context, id uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE clusters
		SET lease_until = NULL
		WHERE id = $1`, id)
	return err
}
