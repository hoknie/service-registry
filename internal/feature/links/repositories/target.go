package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type TargetRepository struct{ db *pgtx.DB }

func NewTargetRepository(db *pgtx.DB) *TargetRepository { return &TargetRepository{db: db} }

func (s *TargetRepository) Seen(ctx context.Context, projectID uuid.UUID, seen linkstypes.Sighting, urlIDs, pairIDs []uuid.UUID) error {
	pairURLs, pairKeys := make([]string, len(seen.Pairs)), make([]string, len(seen.Pairs))
	for i, p := range seen.Pairs {
		pairURLs[i], pairKeys[i] = p.URL, p.LinkKey
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
		DO UPDATE SET last_seen_at = now()`, urlIDs, seen.URLs, projectID, pairIDs, pairURLs, pairKeys)
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

func (s *TargetRepository) LockStatus(ctx context.Context, url string) (*linkstypes.Status, error) {
	previous, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[*string], `
		SELECT last_status
		FROM link_targets
		WHERE url = $1 FOR UPDATE`, url)
	if err != nil || previous == nil || *previous == nil {
		return nil, err
	}
	return new(linkstypes.Status(**previous)), nil
}

func (s *TargetRepository) UpsertChecked(ctx context.Context, id uuid.UUID, url string, r linkstypes.Result, intervalSecs int) (uuid.UUID, string, error) {
	var checkedAt string
	targetID, err := postgres.One(ctx, s.db.From(ctx), constraints, func(row pgx.CollectableRow) (uuid.UUID, error) {
		var id uuid.UUID
		err := row.Scan(&id, &checkedAt)
		return id, err
	}, `
		INSERT INTO link_targets AS t (id, url, last_status, last_http_status, last_duration_ms,
			last_checked_at, next_run_at)
		VALUES ($1, $2, $3, $4, $5, now(), now() + make_interval(secs => $6))
		ON CONFLICT (url)
		DO UPDATE SET last_status = EXCLUDED.last_status, last_http_status = EXCLUDED.last_http_status,
			last_duration_ms = EXCLUDED.last_duration_ms, last_checked_at = now(),
			next_run_at = EXCLUDED.next_run_at, lease_until = NULL
		RETURNING t.id, rfc3339(t.last_checked_at)`, id, url, string(r.Status), r.HTTPStatus, r.DurationMS, intervalSecs)
	return targetID, checkedAt, err
}

func (s *TargetRepository) InsertCheck(ctx context.Context, id, targetID uuid.UUID, r linkstypes.Result) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO link_checks (id, target_id, status, http_status, duration_ms, checked_at)
		VALUES ($1, $2, $3, $4, $5, now())`, id, targetID, string(r.Status), r.HTTPStatus, r.DurationMS)
	return err
}

func (s *TargetRepository) PruneChecks(ctx context.Context, targetID uuid.UUID, keep int) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM link_checks
		WHERE target_id = $1
			AND id NOT IN (
				SELECT id
				FROM link_checks
				WHERE target_id = $1
				ORDER BY checked_at DESC, id DESC
				LIMIT $2
			)`, targetID, keep)
	return err
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
