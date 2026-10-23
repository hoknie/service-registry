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

func (s *TargetRepository) Seen(ctx context.Context, projectID uuid.UUID, links []linkstypes.Link) error {
	type pair struct{ url, key string }
	var pairs []pair
	var urls []string
	for _, l := range links {
		if l.URL == nil {
			continue
		}
		if p := (pair{*l.URL, l.LinkKey}); !slices.Contains(pairs, p) {
			pairs = append(pairs, p)
		}
		if !slices.Contains(urls, *l.URL) {
			urls = append(urls, *l.URL)
		}
	}
	if len(urls) == 0 {
		return nil
	}
	slices.Sort(urls)
	ids := make([]uuid.UUID, len(urls))
	for i := range ids {
		ids[i] = uuid.Must(uuid.NewV7())
	}
	pairIDs, pairURLs, pairKeys := make([]uuid.UUID, len(pairs)), make([]string, len(pairs)), make([]string, len(pairs))
	for i, p := range pairs {
		pairIDs[i], pairURLs[i], pairKeys[i] = uuid.Must(uuid.NewV7()), p.url, p.key
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		WITH t AS (
			INSERT INTO link_targets (id, url)
			SELECT *
			FROM unnest($1::uuid[], $2::text[])
			ON CONFLICT (url)
			DO UPDATE SET last_seen_at = now()
			RETURNING id, url
		)
		INSERT INTO link_target_projects (id, target_id, project_id, link_key)
		SELECT x.id, t.id, $3, x.key
		FROM unnest($4::uuid[], $5::text[], $6::text[]) AS x(id, url, key)
		JOIN t ON t.url = x.url
		ON CONFLICT (target_id, project_id, link_key)
		DO UPDATE SET last_seen_at = now()`, ids, urls, projectID, pairIDs, pairURLs, pairKeys)
	return err
}

func (s *TargetRepository) Projects(ctx context.Context, targetID uuid.UUID) ([]linkstypes.TargetProject, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (linkstypes.TargetProject, error) {
		var p linkstypes.TargetProject
		err := r.Scan(&p.ProjectID, &p.LinkKey)
		return p, err
	}, `
		SELECT project_id, link_key
		FROM link_target_projects
		WHERE target_id = $1
		ORDER BY project_id, link_key`, targetID)
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

func (s *TargetRepository) Record(ctx context.Context, url string, r linkstypes.Result, intervalSecs, history uint32) (linkstypes.Recorded, error) {
	out := linkstypes.Recorded{Check: linkstypes.Check{URL: url, Status: r.Status, HTTPStatus: r.HTTPStatus, DurationMS: r.DurationMS},
		CheckID: uuid.Must(uuid.NewV7())}
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		previous, err := postgres.Find(ctx, tx, constraints, pgx.RowTo[*string], `
			SELECT last_status
			FROM link_targets
			WHERE url = $1 FOR UPDATE`, url)
		if err != nil {
			return err
		}
		if previous != nil && *previous != nil {
			out.Previous = new(linkstypes.Status(**previous))
		}
		out.TargetID, err = postgres.One(ctx, tx, constraints, func(row pgx.CollectableRow) (uuid.UUID, error) {
			var id uuid.UUID
			err := row.Scan(&id, &out.Check.CheckedAt)
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
			VALUES ($1, $2, $3, $4, $5, now())`, out.CheckID, out.TargetID, string(r.Status), r.HTTPStatus, r.DurationMS); err != nil {
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
				)`, out.TargetID, int(history))
		return err
	})
	if err != nil {
		return linkstypes.Recorded{}, postgres.Translate(err, constraints)
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
	if _, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM link_target_projects
		WHERE last_seen_at < now() - make_interval(days => $1::int)`, int(ttlDays)); err != nil {
		return 0, err
	}
	return postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM link_targets
		WHERE last_seen_at < now() - make_interval(days => $1::int)`, int(ttlDays))
}
