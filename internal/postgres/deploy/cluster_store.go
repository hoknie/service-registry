package deploy

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/deploy"
	"svc-registry/internal/forge"
)

type ClusterStore struct{ pool *pgxpool.Pool }

func NewClusterStore(pool *pgxpool.Pool) *ClusterStore { return &ClusterStore{pool: pool} }

func (s *ClusterStore) List(ctx context.Context) ([]domain.Cluster, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.name, c.environment, c.in_cluster, c.api_url, c.ca_pem, c.credentials_ref,
			c.credentials_fingerprint, c.namespaces, c.rules, c.interval_secs, c.enabled, c.status,
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
	if err != nil {
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[clusterRow])
	if err != nil {
		return nil, dbErr(err)
	}
	out := make([]domain.Cluster, 0, len(items))
	for _, r := range items {
		out = append(out, r.cluster())
	}
	return out, nil
}

func (s *ClusterStore) Get(ctx context.Context, id uuid.UUID) (*domain.Cluster, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.name, c.environment, c.in_cluster, c.api_url, c.ca_pem, c.credentials_ref,
			c.credentials_fingerprint, c.namespaces, c.rules, c.interval_secs, c.enabled, c.status,
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
	if err != nil {
		return nil, dbErr(err)
	}
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[clusterRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	c := r.cluster()
	return &c, nil
}

func rulesParam(rules []domain.Rule) []domain.Rule {
	if rules == nil {
		return []domain.Rule{}
	}
	return rules
}

func namespacesParam(ns []string) []string {
	if ns == nil {
		return []string{}
	}
	return ns
}

func (s *ClusterStore) Insert(ctx context.Context, c domain.NewCluster) (domain.Cluster, error) {
	st := c.Settings
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO clusters (id, name, environment, in_cluster, api_url, ca_pem, credentials_enc,
			credentials_ref, credentials_fingerprint, namespaces, rules, interval_secs, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12, $13)`,
		c.ID, st.Name, st.Environment, st.InCluster, st.APIURL, st.CAPEM,
		c.Credentials.Enc, c.Credentials.Ref, c.Credentials.Fingerprint, namespacesParam(st.Namespaces), rulesParam(st.Rules),
		st.IntervalSecs, st.Enabled); err != nil {
		return domain.Cluster{}, dbErr(err)
	}
	got, err := s.Get(ctx, c.ID)
	if err != nil {
		return domain.Cluster{}, err
	}
	if got == nil {
		return domain.Cluster{}, &domain.InternalError{Detail: "inserted cluster vanished"}
	}
	return *got, nil
}

func (s *ClusterStore) Update(ctx context.Context, id uuid.UUID, u domain.ClusterUpdate) (*domain.Cluster, error) {
	st := u.Settings
	creds := forge.StoredCredentials{}
	if u.Credentials != nil {
		creds = *u.Credentials
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE clusters
		SET name = $2, environment = $3, in_cluster = $4, api_url = $5, ca_pem = $6, namespaces = $7,
			rules = $8::jsonb, interval_secs = $9, enabled = $10,
			credentials_enc = CASE WHEN $4 THEN NULL WHEN $11 THEN $12 ELSE credentials_enc END,
			credentials_ref = CASE WHEN $4 THEN NULL WHEN $11 THEN $13 ELSE credentials_ref END,
			credentials_fingerprint = CASE WHEN $4 THEN NULL WHEN $11 THEN $14 ELSE credentials_fingerprint END,
			updated_at = now()
		WHERE id = $1`, id, st.Name, st.Environment, st.InCluster, st.APIURL, st.CAPEM,
		namespacesParam(st.Namespaces), rulesParam(st.Rules), st.IntervalSecs, st.Enabled,
		u.Credentials != nil, creds.Enc, creds.Ref, creds.Fingerprint)
	if err != nil {
		return nil, dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return nil, nil
	}
	return s.Get(ctx, id)
}

func (s *ClusterStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM clusters
		WHERE id = $1`, id)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *ClusterStore) Stored(ctx context.Context, id uuid.UUID) (*forge.StoredCredentials, error) {
	var c forge.StoredCredentials
	err := s.pool.QueryRow(ctx, `
		SELECT credentials_enc, credentials_ref, credentials_fingerprint
		FROM clusters
		WHERE id = $1`, id).Scan(&c.Enc, &c.Ref, &c.Fingerprint)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &c, nil
}

func (s *ClusterStore) ScheduleNow(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE clusters
		SET next_run_at = now()
		WHERE id = $1`, id)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *ClusterStore) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx, `
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
	if err != nil {
		return nil, dbErr(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	return ids, dbErr(err)
}

func (s *ClusterStore) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE clusters
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id = $1`, id, leaseSecs)
	return dbErr(err)
}

func (s *ClusterStore) Finish(ctx context.Context, id uuid.UUID, failure *domain.Failure) error {
	status := string(domain.StatusOK)
	var lastError any
	if failure != nil {
		status = string(domain.StatusError)
		lastError = errorJSON{Code: string(failure.Code), Message: failure.Message}
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE clusters
		SET status = $2, last_error = $3::jsonb, last_polled_at = now(),
			next_run_at = now() + make_interval(secs => interval_secs), lease_until = NULL
		WHERE id = $1`, id, status, lastError)
	return dbErr(err)
}

func (s *ClusterStore) Secrets(ctx context.Context) ([]domain.StoredSecret, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, credentials_enc
		FROM clusters
		WHERE credentials_enc IS NOT NULL
		ORDER BY id`)
	if err != nil {
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.StoredSecret, error) {
		var v domain.StoredSecret
		err := r.Scan(&v.ID, &v.Enc)
		return v, err
	})
	return items, dbErr(err)
}

func (s *ClusterStore) ReplaceSecret(ctx context.Context, id uuid.UUID, enc string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE clusters
		SET credentials_enc = $2
		WHERE id = $1`, id, enc)
	return dbErr(err)
}
