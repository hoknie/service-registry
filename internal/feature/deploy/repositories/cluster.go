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

type ClusterRepository struct{ db *pgtx.DB }

func NewClusterRepository(db *pgtx.DB) *ClusterRepository { return &ClusterRepository{db: db} }

func (s *ClusterRepository) List(ctx context.Context) ([]deploytypes.Cluster, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(clusterRow.cluster), `
		SELECT c.id, c.name, c.environment, c.in_cluster, c.api_url, c.ca_pem, c.credentials_secret_id,
			c.credentials_enc IS NOT NULL AS has_enc, c.credentials_ref, c.credentials_fingerprint, c.namespaces, c.rules, c.interval_secs, c.enabled, c.status,
			c.last_error, rfc3339(c.last_polled_at) AS last_polled_at, rfc3339(c.next_run_at) AS next_run_at, (
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
		ORDER BY lower(c.name), c.id`)
}

func (s *ClusterRepository) Get(ctx context.Context, id uuid.UUID) (*deploytypes.Cluster, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, postgres.Struct(clusterRow.cluster), `
		SELECT c.id, c.name, c.environment, c.in_cluster, c.api_url, c.ca_pem, c.credentials_secret_id,
			c.credentials_enc IS NOT NULL AS has_enc, c.credentials_ref, c.credentials_fingerprint, c.namespaces, c.rules, c.interval_secs, c.enabled, c.status,
			c.last_error, rfc3339(c.last_polled_at) AS last_polled_at, rfc3339(c.next_run_at) AS next_run_at, (
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
		WHERE c.id = $1`, id)
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
		INSERT INTO clusters (id, name, environment, in_cluster, api_url, ca_pem, credentials_secret_id,
			namespaces, rules, interval_secs, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11)`,
		c.ID, st.Name, st.Environment, st.InCluster, st.APIURL, st.CAPEM,
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
			credentials_enc = CASE WHEN $4 OR $11 THEN NULL ELSE credentials_enc END,
			credentials_ref = CASE WHEN $4 OR $11 THEN NULL ELSE credentials_ref END,
			credentials_fingerprint = CASE WHEN $4 OR $11 THEN NULL ELSE credentials_fingerprint END,
			credentials_secret_id = CASE WHEN $4 THEN NULL WHEN $11 THEN $12 ELSE credentials_secret_id END,
			updated_at = now()
		WHERE id = $1`, id, st.Name, st.Environment, st.InCluster, st.APIURL, st.CAPEM,
		namespacesParam(st.Namespaces), rulesParam(st.Rules), st.IntervalSecs, st.Enabled,
		u.SecretID != nil, u.SecretID)
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
		err := r.Scan(&c.SecretID, &c.Enc, &c.Ref)
		return c, err
	}, `
		SELECT credentials_secret_id, credentials_enc, credentials_ref
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

func (s *ClusterRepository) Secrets(ctx context.Context) ([]deploytypes.StoredSecret, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (deploytypes.StoredSecret, error) {
		var v deploytypes.StoredSecret
		err := r.Scan(&v.ID, &v.Enc)
		return v, err
	}, `
		SELECT id, credentials_enc
		FROM clusters
		WHERE credentials_enc IS NOT NULL
		ORDER BY id`)
}

func (s *ClusterRepository) ReplaceSecret(ctx context.Context, id uuid.UUID, enc string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE clusters
		SET credentials_enc = $2
		WHERE id = $1`, id, enc)
	return err
}

func (s *ClusterRepository) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return postgres.Pairs[uuid.UUID, int64](ctx, s.db.From(ctx), constraints, `
		SELECT credentials_secret_id, count(*)
		FROM clusters
		WHERE credentials_secret_id = ANY($1)
		GROUP BY credentials_secret_id`, secrets)
}
