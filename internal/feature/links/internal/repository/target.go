package repository

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/links"
	"svc-registry/internal/platform/postgres"
)

type Targets struct{ db *postgres.DB }

func NewTargets(db *postgres.DB) *Targets { return &Targets{db: db} }

func (s *Targets) Seen(ctx context.Context, urls []string) error {
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
	_, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO link_targets (id, url)
		SELECT *
		FROM unnest($1::uuid[], $2::text[])
		ON CONFLICT (url)
		DO UPDATE SET last_seen_at = now()`, ids, urls)
	return dbErr(err)
}

func (s *Targets) Latest(ctx context.Context, urls []string) (map[string]links.Check, error) {
	out := map[string]links.Check{}
	if len(urls) == 0 {
		return out, nil
	}
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT t.url, t.last_status AS status, t.last_http_status AS http_status,
			t.last_duration_ms AS duration_ms, rfc3339(t.last_checked_at) AS checked_at
		FROM link_targets t
		WHERE t.url = ANY($1)
			AND t.last_checked_at IS NOT NULL`, urls)
	if err != nil {
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[checkRow])
	if err != nil {
		return nil, dbErr(err)
	}
	for _, r := range items {
		out[r.URL] = r.check()
	}
	return out, nil
}

func (s *Targets) Fresh(ctx context.Context, url string, withinSecs int) (*links.Check, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT t.url, t.last_status AS status, t.last_http_status AS http_status,
			t.last_duration_ms AS duration_ms, rfc3339(t.last_checked_at) AS checked_at
		FROM link_targets t
		WHERE t.url = $1
			AND t.last_checked_at > now() - make_interval(secs => $2)`, url, withinSecs)
	if err != nil {
		return nil, dbErr(err)
	}
	r, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[checkRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	c := r.check()
	return &c, nil
}

func (s *Targets) Record(ctx context.Context, url string, r links.Result, intervalSecs, history uint32) (links.Check, error) {
	out := links.Check{URL: url, Status: r.Status, HTTPStatus: r.HTTPStatus, DurationMS: r.DurationMS}
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		var id uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO link_targets AS t (id, url, last_status, last_http_status, last_duration_ms,
				last_checked_at, next_run_at)
			VALUES ($1, $2, $3, $4, $5, now(), now() + make_interval(secs => $6))
			ON CONFLICT (url)
			DO UPDATE SET last_status = EXCLUDED.last_status, last_http_status = EXCLUDED.last_http_status,
				last_duration_ms = EXCLUDED.last_duration_ms, last_checked_at = now(),
				next_run_at = EXCLUDED.next_run_at, lease_until = NULL
			RETURNING t.id, rfc3339(t.last_checked_at)`, uuid.Must(uuid.NewV7()), url, string(r.Status), r.HTTPStatus,
			r.DurationMS, int(intervalSecs)).Scan(&id, &out.CheckedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO link_checks (id, target_id, status, http_status, duration_ms, checked_at)
			VALUES ($1, $2, $3, $4, $5, now())`, uuid.Must(uuid.NewV7()), id, string(r.Status), r.HTTPStatus, r.DurationMS); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
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
		return links.Check{}, dbErr(err)
	}
	return out, nil
}

func (s *Targets) History(ctx context.Context, urls []string) ([]links.Check, error) {
	if len(urls) == 0 {
		return []links.Check{}, nil
	}
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT t.url, c.status, c.http_status, c.duration_ms, rfc3339(c.checked_at) AS checked_at
		FROM link_checks c
		JOIN link_targets t ON t.id = c.target_id
		WHERE t.url = ANY($1)
		ORDER BY c.checked_at DESC, c.id DESC`, urls)
	if err != nil {
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[checkRow])
	if err != nil {
		return nil, dbErr(err)
	}
	out := make([]links.Check, 0, len(items))
	for _, r := range items {
		out = append(out, r.check())
	}
	return out, nil
}

func (s *Targets) Claim(ctx context.Context, limit, leaseSecs int) ([]links.Target, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
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
	if err != nil {
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (links.Target, error) {
		var t links.Target
		err := r.Scan(&t.ID, &t.URL)
		return t, err
	})
	return items, dbErr(err)
}

func (s *Targets) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE link_targets
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id = $1`, id, leaseSecs)
	return dbErr(err)
}

func (s *Targets) Prune(ctx context.Context, ttlDays uint32) (int64, error) {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM link_targets
		WHERE last_seen_at < now() - make_interval(days => $1::int)`, int(ttlDays))
	if err != nil {
		return 0, dbErr(err)
	}
	return tag.RowsAffected(), nil
}
