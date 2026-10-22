package repositories

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type TargetRepository struct{ db *pgtx.DB }

func NewTargetRepository(db *pgtx.DB) *TargetRepository { return &TargetRepository{db: db} }

func (s *TargetRepository) Seen(ctx context.Context, urls []string) error {
	if len(urls) == 0 {
		return nil
	}
	urls = slices.Clone(urls)
	slices.Sort(urls)
	urls = slices.Compact(urls)
	ids := make([]uuid.UUID, len(urls))
	for i := range ids {
		ids[i] = uuid.Must(uuid.NewV7())
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO link_targets (id, url)
		SELECT *
		FROM unnest($1::uuid[], $2::text[])
		ON CONFLICT (url)
		DO UPDATE SET last_seen_at = now()`, ids, urls)
	return err
}

func (s *TargetRepository) Latest(ctx context.Context, urls []string) (map[string]linkstypes.Check, error) {
	out := map[string]linkstypes.Check{}
	if len(urls) == 0 {
		return out, nil
	}
	items, err := postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowToStructByName[checkRow], `
		SELECT t.url, t.last_status AS status, t.last_http_status AS http_status,
			t.last_duration_ms AS duration_ms, rfc3339(t.last_checked_at) AS checked_at
		FROM link_targets t
		WHERE t.url = ANY($1)
			AND t.last_checked_at IS NOT NULL`, urls)
	if err != nil {
		return nil, err
	}
	for _, r := range items {
		out[r.URL] = r.check()
	}
	return out, nil
}

func (s *TargetRepository) Fresh(ctx context.Context, url string, withinSecs int) (*linkstypes.Check, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, postgres.Struct(checkRow.check), `
		SELECT t.url, t.last_status AS status, t.last_http_status AS http_status,
			t.last_duration_ms AS duration_ms, rfc3339(t.last_checked_at) AS checked_at
		FROM link_targets t
		WHERE t.url = $1
			AND t.last_checked_at > now() - make_interval(secs => $2)`, url, withinSecs)
}

func (s *TargetRepository) Record(ctx context.Context, url string, r linkstypes.Result, intervalSecs, history uint32) (linkstypes.Check, error) {
	out := linkstypes.Check{URL: url, Status: r.Status, HTTPStatus: r.HTTPStatus, DurationMS: r.DurationMS}
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		id, err := postgres.One(ctx, tx, constraints, func(row pgx.CollectableRow) (uuid.UUID, error) {
			var id uuid.UUID
			err := row.Scan(&id, &out.CheckedAt)
			return id, err
		}, `
			INSERT INTO link_targets AS t (id, url, last_status, last_http_status, last_duration_ms,
				last_checked_at, next_run_at)
			VALUES ($1, $2, $3, $4, $5, now(), now() + make_interval(secs => $6))
			ON CONFLICT (url)
			DO UPDATE SET last_status = EXCLUDED.last_status, last_http_status = EXCLUDED.last_http_status,
				last_duration_ms = EXCLUDED.last_duration_ms, last_checked_at = now(),
				next_run_at = EXCLUDED.next_run_at, lease_until = NULL
			RETURNING t.id, rfc3339(t.last_checked_at)`, uuid.Must(uuid.NewV7()), url, string(r.Status), r.HTTPStatus,
			r.DurationMS, int(intervalSecs))
		if err != nil {
			return err
		}
		if _, err := postgres.Exec(ctx, tx, constraints, `
			INSERT INTO link_checks (id, target_id, status, http_status, duration_ms, checked_at)
			VALUES ($1, $2, $3, $4, $5, now())`, uuid.Must(uuid.NewV7()), id, string(r.Status), r.HTTPStatus, r.DurationMS); err != nil {
			return err
		}
		_, err = postgres.Exec(ctx, tx, constraints, `
			DELETE FROM link_checks
			WHERE target_id = $1
				AND id NOT IN (
					SELECT id
					FROM link_checks
					WHERE target_id = $1
					ORDER BY checked_at DESC, id DESC
					LIMIT $2
				)`, id, int(history))
		return err
	})
	if err != nil {
		return linkstypes.Check{}, postgres.Translate(err, constraints)
	}
	return out, nil
}

func (s *TargetRepository) History(ctx context.Context, urls []string) ([]linkstypes.Check, error) {
	if len(urls) == 0 {
		return []linkstypes.Check{}, nil
	}
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(checkRow.check), `
		SELECT t.url, c.status, c.http_status, c.duration_ms, rfc3339(c.checked_at) AS checked_at
		FROM link_checks c
		JOIN link_targets t ON t.id = c.target_id
		WHERE t.url = ANY($1)
		ORDER BY c.checked_at DESC, c.id DESC`, urls)
}

func (s *TargetRepository) Claim(ctx context.Context, limit, leaseSecs int) ([]linkstypes.Target, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (linkstypes.Target, error) {
		var t linkstypes.Target
		err := r.Scan(&t.ID, &t.URL)
		return t, err
	}, `
		UPDATE link_targets
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id IN (
			SELECT id
			FROM link_targets
			WHERE next_run_at <= now()
				AND (lease_until IS NULL OR lease_until < now())
			ORDER BY next_run_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		RETURNING id, url`, limit, leaseSecs)
}

func (s *TargetRepository) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE link_targets
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id = $1`, id, leaseSecs)
	return err
}

func (s *TargetRepository) Prune(ctx context.Context, ttlDays uint32) (int64, error) {
	return postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM link_targets
		WHERE last_seen_at < now() - make_interval(days => $1::int)`, int(ttlDays))
}
