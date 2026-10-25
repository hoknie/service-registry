package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/postgres"
)

func (s *ConnectionRepository) ScheduleNow(ctx context.Context, id uuid.UUID, trigger forgetypes.Trigger) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE forge_connections
		SET next_run_at = now(), next_trigger = $2
		WHERE id = $1`, id, string(trigger))
}

func (s *ConnectionRepository) Claim(ctx context.Context, limit, leaseSecs int) ([]forgetypes.Claimed, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (forgetypes.Claimed, error) {
		var c forgetypes.Claimed
		var trigger string
		err := r.Scan(&c.ID, &trigger)
		c.Trigger = forgetypes.Trigger(trigger)
		return c, err
	}, `
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
}

func (s *ConnectionRepository) ClaimOne(ctx context.Context, id uuid.UUID, leaseSecs int) (bool, error) {
	got, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		UPDATE forge_connections
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id = (
			SELECT id
			FROM forge_connections
			WHERE id = $1
				AND (lease_until IS NULL OR lease_until < now())
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id`, id, float64(leaseSecs))
	return got != nil, err
}

func (s *ConnectionRepository) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE forge_connections
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id = $1`, id, float64(leaseSecs))
	return err
}

func (s *ConnectionRepository) Release(ctx context.Context, id uuid.UUID, next *time.Time) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE forge_connections
		SET lease_until = NULL, next_trigger = 'schedule',
			next_run_at = COALESCE($2, now() + make_interval(secs => interval_secs))
		WHERE id = $1`, id, next)
	return err
}

func (s *ConnectionRepository) MetricCounts(ctx context.Context) (map[string]int64, error) {
	return postgres.Pairs[string, int64](ctx, s.db.From(ctx), constraints, `
		SELECT 'connections:' || c.kind || '/' || CASE WHEN l.status = 'failed' THEN 'failed' ELSE 'ok' END, count(*)
		FROM forge_connections c
		LEFT JOIN LATERAL (
			SELECT w.status
			FROM forge_sync_runs w
			WHERE w.connection_id = c.id
				AND w.status <> 'running'
			ORDER BY w.started_at DESC, w.id DESC
			LIMIT 1
		) l ON true
		GROUP BY 1
		UNION ALL
		SELECT 'covered:forge', count(*)
		FROM forge_repositories
		WHERE orphaned_at IS NULL`)
}

func (s *ConnectionRepository) QueueCounts(ctx context.Context) (map[string]int64, error) {
	return postgres.Pairs[string, int64](ctx, s.db.From(ctx), constraints, `
		SELECT 'queue:forge_sync/' || CASE WHEN lease_until > now() THEN 'leased' ELSE 'due' END, count(*)
		FROM forge_connections
		WHERE lease_until > now()
			OR next_run_at <= now()
		GROUP BY 1`)
}
