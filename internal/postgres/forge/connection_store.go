package forge

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/forge"
)

type ConnectionStore struct{ pool *pgxpool.Pool }

func NewConnectionStore(pool *pgxpool.Pool) *ConnectionStore { return &ConnectionStore{pool: pool} }

func (s *ConnectionStore) Insert(ctx context.Context, n domain.NewConnection) (domain.Connection, error) {
	st := n.Settings
	_, err := s.pool.Exec(ctx, `
		INSERT INTO forge_connections (id, node_id, kind, api_url, owner_path, mirror_subgroups,
			include_archived, include_forks, name_include, name_exclude, interval_secs, credentials_enc,
			credentials_ref, credentials_fingerprint, branch_include)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		n.ID, n.NodeID, string(st.Kind), st.APIURL, st.OwnerPath, st.MirrorSubgroups,
		st.IncludeArchived, st.IncludeForks, st.NameInclude, st.NameExclude, st.IntervalSecs,
		n.Credentials.Enc, n.Credentials.Ref, n.Credentials.Fingerprint, nonNil(st.BranchInclude))
	if err != nil {
		return domain.Connection{}, dbErr(err)
	}
	return s.must(ctx, n.ID)
}

func (s *ConnectionStore) must(ctx context.Context, id uuid.UUID) (domain.Connection, error) {
	c, err := s.Find(ctx, id)
	if err != nil {
		return domain.Connection{}, err
	}
	if c == nil {
		return domain.Connection{}, domain.ErrNotFound
	}
	return *c, nil
}

func (s *ConnectionStore) List(ctx context.Context, nodeID uuid.UUID) ([]domain.Connection, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.node_id, c.kind, c.api_url, c.owner_path, c.mirror_subgroups, c.include_archived,
			c.include_forks, c.name_include, c.name_exclude, c.branch_include, c.interval_secs,
			c.credentials_ref, c.credentials_fingerprint, c.webhook_mode, rfc3339(c.next_run_at),
			rfc3339(c.created_at), rfc3339(c.updated_at), lr.id, lr.connection_id, lr.trigger, lr.status,
			rfc3339(lr.started_at), rfc3339(lr.finished_at), lr.created, lr.updated, lr.orphaned, lr.skipped,
			lr.error_code, lr.error_message, lr.problems
		FROM forge_connections c
		LEFT JOIN LATERAL (
			SELECT *
			FROM forge_sync_runs r
			WHERE r.connection_id = c.id
			ORDER BY r.started_at DESC, r.id DESC
			LIMIT 1
		) lr ON true
		WHERE c.node_id = $1
		ORDER BY c.created_at, c.id`, nodeID)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Connection, error) { return scanConnection(r) })
	return out, dbErr(err)
}

func (s *ConnectionStore) Find(ctx context.Context, id uuid.UUID) (*domain.Connection, error) {
	c, err := scanConnection(s.pool.QueryRow(ctx, `
		SELECT c.id, c.node_id, c.kind, c.api_url, c.owner_path, c.mirror_subgroups, c.include_archived,
			c.include_forks, c.name_include, c.name_exclude, c.branch_include, c.interval_secs,
			c.credentials_ref, c.credentials_fingerprint, c.webhook_mode, rfc3339(c.next_run_at),
			rfc3339(c.created_at), rfc3339(c.updated_at), lr.id, lr.connection_id, lr.trigger, lr.status,
			rfc3339(lr.started_at), rfc3339(lr.finished_at), lr.created, lr.updated, lr.orphaned, lr.skipped,
			lr.error_code, lr.error_message, lr.problems
		FROM forge_connections c
		LEFT JOIN LATERAL (
			SELECT *
			FROM forge_sync_runs r
			WHERE r.connection_id = c.id
			ORDER BY r.started_at DESC, r.id DESC
			LIMIT 1
		) lr ON true
		WHERE c.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &c, nil
}

func (s *ConnectionStore) Secrets(ctx context.Context, id uuid.UUID) (*domain.Secrets, error) {
	var sec domain.Secrets
	err := s.pool.QueryRow(ctx, `
		SELECT credentials_enc, credentials_ref, webhook_secret_enc, webhook_id
		FROM forge_connections
		WHERE id = $1`, id).Scan(&sec.CredentialsEnc, &sec.CredentialsRef, &sec.WebhookSecretEnc, &sec.WebhookID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &sec, nil
}

func (s *ConnectionStore) Update(ctx context.Context, id uuid.UUID, st domain.Settings, creds *domain.StoredCredentials) (domain.Connection, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE forge_connections
			SET api_url = $2, owner_path = $3, mirror_subgroups = $4, include_archived = $5, include_forks = $6,
				name_include = $7, name_exclude = $8, interval_secs = $9, branch_include = $10, updated_at = now()
			WHERE id = $1`, id, st.APIURL, st.OwnerPath, st.MirrorSubgroups, st.IncludeArchived,
			st.IncludeForks, st.NameInclude, st.NameExclude, st.IntervalSecs, nonNil(st.BranchInclude))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		if creds != nil {
			_, err = tx.Exec(ctx, `
				UPDATE forge_connections
				SET credentials_enc = $2, credentials_ref = $3, credentials_fingerprint = $4, updated_at = now()
				WHERE id = $1`, id, creds.Enc, creds.Ref, creds.Fingerprint)
		}
		return err
	})
	if err != nil {
		return domain.Connection{}, dbErr(err)
	}
	return s.must(ctx, id)
}

func (s *ConnectionStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM forge_connections
		WHERE id = $1`, id)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *ConnectionStore) SetWebhook(ctx context.Context, id uuid.UUID, h domain.Webhook) error {
	var mode *string
	if h.Mode != nil {
		m := string(*h.Mode)
		mode = &m
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE forge_connections
		SET webhook_mode = $2, webhook_secret_enc = $3, webhook_id = $4, updated_at = now()
		WHERE id = $1`, id, mode, h.SecretEnc, h.HookID)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *ConnectionStore) ScheduleNow(ctx context.Context, id uuid.UUID, trigger domain.Trigger) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE forge_connections
		SET next_run_at = now(), next_trigger = $2
		WHERE id = $1`, id, string(trigger))
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *ConnectionStore) Claim(ctx context.Context, limit, leaseSecs int) ([]domain.Claimed, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE forge_connections
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id IN (
			SELECT id
			FROM forge_connections
			WHERE next_run_at <= now()
				AND (lease_until IS NULL OR lease_until < now())
			ORDER BY next_run_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		RETURNING id, next_trigger`, limit, float64(leaseSecs))
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Claimed, error) {
		var c domain.Claimed
		var trigger string
		err := r.Scan(&c.ID, &trigger)
		c.Trigger = domain.Trigger(trigger)
		return c, err
	})
	return out, dbErr(err)
}

func (s *ConnectionStore) ClaimOne(ctx context.Context, id uuid.UUID, leaseSecs int) (bool, error) {
	var got uuid.UUID
	err := s.pool.QueryRow(ctx, `
		UPDATE forge_connections
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id = (
			SELECT id
			FROM forge_connections
			WHERE id = $1
				AND (lease_until IS NULL OR lease_until < now())
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id`, id, float64(leaseSecs)).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, dbErr(err)
}

func (s *ConnectionStore) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE forge_connections
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id = $1`, id, float64(leaseSecs))
	return dbErr(err)
}

func (s *ConnectionStore) Release(ctx context.Context, id uuid.UUID, next *time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE forge_connections
		SET lease_until = NULL, next_trigger = 'schedule',
			next_run_at = COALESCE($2, now() + make_interval(secs => interval_secs))
		WHERE id = $1`, id, next)
	return dbErr(err)
}

func (s *ConnectionStore) AllSecrets(ctx context.Context) ([]domain.SecretRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, credentials_enc, webhook_secret_enc
		FROM forge_connections
		ORDER BY id`)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.SecretRow, error) {
		var row domain.SecretRow
		err := r.Scan(&row.ID, &row.CredentialsEnc, &row.WebhookSecretEnc)
		return row, err
	})
	return out, dbErr(err)
}

func (s *ConnectionStore) ReplaceSecrets(ctx context.Context, row domain.SecretRow) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE forge_connections
		SET credentials_enc = $2, webhook_secret_enc = $3
		WHERE id = $1`, row.ID, row.CredentialsEnc, row.WebhookSecretEnc)
	return dbErr(err)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
