package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/forge"
	"svc-registry/internal/platform/postgres"
)

type Connections struct{ db *postgres.DB }

func NewConnections(db *postgres.DB) *Connections {
	return &Connections{db: db}
}

func (s *Connections) Insert(ctx context.Context, n forge.NewConnection) (forge.Connection, error) {
	st := n.Settings
	_, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO forge_connections (id, node_id, kind, api_url, owner_path, mirror_subgroups,
			include_archived, include_forks, name_include, name_exclude, interval_secs, credentials_secret_id,
			branch_include)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		n.ID, n.NodeID, string(st.Kind), st.APIURL, st.OwnerPath, st.MirrorSubgroups,
		st.IncludeArchived, st.IncludeForks, st.NameInclude, st.NameExclude, st.IntervalSecs,
		n.SecretID, nonNil(st.BranchInclude))
	if err != nil {
		return forge.Connection{}, dbErr(err)
	}
	return s.must(ctx, n.ID)
}

func (s *Connections) must(ctx context.Context, id uuid.UUID) (forge.Connection, error) {
	c, err := s.Find(ctx, id)
	if err != nil {
		return forge.Connection{}, err
	}
	if c == nil {
		return forge.Connection{}, forge.ErrNotFound
	}
	return *c, nil
}

func (s *Connections) List(ctx context.Context, nodeID uuid.UUID) ([]forge.Connection, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT c.id, c.node_id, c.kind, c.api_url, c.owner_path, c.mirror_subgroups, c.include_archived,
			c.include_forks, c.name_include, c.name_exclude, c.branch_include, c.interval_secs,
			c.credentials_secret_id, c.credentials_enc IS NOT NULL, c.credentials_ref, c.credentials_fingerprint, c.webhook_mode, rfc3339(c.next_run_at),
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
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (forge.Connection, error) { return scanConnection(r) })
	return out, dbErr(err)
}

func (s *Connections) Find(ctx context.Context, id uuid.UUID) (*forge.Connection, error) {
	c, err := scanConnection(s.db.From(ctx).QueryRow(ctx, `
		SELECT c.id, c.node_id, c.kind, c.api_url, c.owner_path, c.mirror_subgroups, c.include_archived,
			c.include_forks, c.name_include, c.name_exclude, c.branch_include, c.interval_secs,
			c.credentials_secret_id, c.credentials_enc IS NOT NULL, c.credentials_ref, c.credentials_fingerprint, c.webhook_mode, rfc3339(c.next_run_at),
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

func (s *Connections) Secrets(ctx context.Context, id uuid.UUID) (*forge.Secrets, error) {
	var sec forge.Secrets
	err := s.db.From(ctx).QueryRow(ctx, `
		SELECT credentials_secret_id, credentials_enc, credentials_ref, webhook_secret_enc, webhook_id
		FROM forge_connections
		WHERE id = $1`, id).Scan(&sec.CredentialsSecretID, &sec.CredentialsEnc, &sec.CredentialsRef, &sec.WebhookSecretEnc, &sec.WebhookID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &sec, nil
}

func (s *Connections) Update(ctx context.Context, id uuid.UUID, st forge.Settings, secret *uuid.UUID) (forge.Connection, error) {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
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
			return forge.ErrNotFound
		}
		if secret != nil {
			_, err = tx.Exec(ctx, `
				UPDATE forge_connections
				SET credentials_secret_id = $2, credentials_enc = NULL, credentials_ref = NULL, credentials_fingerprint = NULL,
					updated_at = now()
				WHERE id = $1`, id, *secret)
		}
		return err
	})
	if err != nil {
		return forge.Connection{}, dbErr(err)
	}
	return s.must(ctx, id)
}

func (s *Connections) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM forge_connections
		WHERE id = $1`, id)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return forge.ErrNotFound
	}
	return nil
}

func (s *Connections) SetWebhook(ctx context.Context, id uuid.UUID, h forge.Webhook) error {
	var mode *string
	if h.Mode != nil {
		m := string(*h.Mode)
		mode = &m
	}
	tag, err := s.db.From(ctx).Exec(ctx, `
		UPDATE forge_connections
		SET webhook_mode = $2, webhook_secret_enc = $3, webhook_id = $4, updated_at = now()
		WHERE id = $1`, id, mode, h.SecretEnc, h.HookID)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return forge.ErrNotFound
	}
	return nil
}

func (s *Connections) ScheduleNow(ctx context.Context, id uuid.UUID, trigger forge.Trigger) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		UPDATE forge_connections
		SET next_run_at = now(), next_trigger = $2
		WHERE id = $1`, id, string(trigger))
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return forge.ErrNotFound
	}
	return nil
}

func (s *Connections) Claim(ctx context.Context, limit, leaseSecs int) ([]forge.Claimed, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
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
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (forge.Claimed, error) {
		var c forge.Claimed
		var trigger string
		err := r.Scan(&c.ID, &trigger)
		c.Trigger = forge.Trigger(trigger)
		return c, err
	})
	return out, dbErr(err)
}

func (s *Connections) ClaimOne(ctx context.Context, id uuid.UUID, leaseSecs int) (bool, error) {
	var got uuid.UUID
	err := s.db.From(ctx).QueryRow(ctx, `
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

func (s *Connections) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE forge_connections
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id = $1`, id, float64(leaseSecs))
	return dbErr(err)
}

func (s *Connections) Release(ctx context.Context, id uuid.UUID, next *time.Time) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE forge_connections
		SET lease_until = NULL, next_trigger = 'schedule',
			next_run_at = COALESCE($2, now() + make_interval(secs => interval_secs))
		WHERE id = $1`, id, next)
	return dbErr(err)
}

func (s *Connections) AllSecrets(ctx context.Context) ([]forge.SecretRow, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT id, credentials_enc, webhook_secret_enc
		FROM forge_connections
		ORDER BY id`)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (forge.SecretRow, error) {
		var row forge.SecretRow
		err := r.Scan(&row.ID, &row.CredentialsEnc, &row.WebhookSecretEnc)
		return row, err
	})
	return out, dbErr(err)
}

func (s *Connections) ReplaceSecrets(ctx context.Context, row forge.SecretRow) error {
	_, err := s.db.From(ctx).Exec(ctx, `
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

func (s *Connections) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT credentials_secret_id, count(*)
		FROM forge_connections
		WHERE credentials_secret_id = ANY($1)
		GROUP BY credentials_secret_id`, secrets)
	if err != nil {
		return nil, dbErr(err)
	}
	out := map[uuid.UUID]int64{}
	var id uuid.UUID
	var n int64
	_, err = pgx.ForEachRow(rows, []any{&id, &n}, func() error {
		out[id] = n
		return nil
	})
	return out, dbErr(err)
}
