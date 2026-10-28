package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type CheckRepository struct{ db *pgtx.DB }

func NewCheckRepository(db *pgtx.DB) *CheckRepository { return &CheckRepository{db: db} }

func (s *CheckRepository) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		UPDATE mcp_server_checks
		SET lease_until = now() + make_interval(secs => $2)
		WHERE server_id IN (
			SELECT c.server_id
			FROM mcp_server_checks c
			JOIN mcp_servers s ON s.id = c.server_id
			WHERE c.next_check_at <= now()
				AND (c.lease_until IS NULL OR c.lease_until < now())
				AND s.status <> 'disabled'
			ORDER BY c.next_check_at
			FOR UPDATE OF c SKIP LOCKED
			LIMIT $1
		)
		RETURNING server_id`, limit, float64(leaseSecs))
}

func (s *CheckRepository) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE mcp_server_checks
		SET lease_until = now() + make_interval(secs => $2)
		WHERE server_id = $1`, id, float64(leaseSecs))
	return err
}

func (s *CheckRepository) TakeNow(ctx context.Context, id uuid.UUID, leaseSecs int) (bool, error) {
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		UPDATE mcp_server_checks
		SET lease_until = now() + make_interval(secs => $2)
		WHERE server_id = $1
			AND (checked_at IS NULL OR checked_at < now() - interval '10 seconds')
			AND (lease_until IS NULL OR lease_until < now())
		RETURNING true`, id, float64(leaseSecs))
	return found != nil, err
}

func (s *CheckRepository) Record(ctx context.Context, rec gatewaytypes.CheckRecord, started time.Time, intervalSecs int) error {
	var info, tools any
	var iconData []byte
	var iconType *string
	if rec.Icon != nil {
		iconData, iconType = rec.Icon.Data, &rec.Icon.Type
	}
	if rec.Outcome == gatewaytypes.OutcomeOK {
		info, tools = rec.Info, rec.Tools
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE mcp_server_checks
		SET checked_at = now(), outcome = $2, code = $3, detail = $4, duration_ms = $5, truncated = $6, lease_until = NULL,
			snapshot_at = CASE WHEN $2 = 'ok' THEN now() ELSE snapshot_at END,
			server_info = CASE WHEN $2 = 'ok' THEN $7::jsonb ELSE server_info END,
			tools = CASE WHEN $2 = 'ok' THEN $8::jsonb ELSE tools END,
			icon = CASE WHEN $2 = 'ok' AND $11::bytea IS NOT NULL THEN $11 ELSE icon END,
			icon_type = CASE WHEN $2 = 'ok' AND $11::bytea IS NOT NULL THEN $12 ELSE icon_type END,
			next_check_at = CASE WHEN next_check_at > $9 THEN next_check_at ELSE now() + make_interval(secs => $10) END
		WHERE server_id = $1`, rec.ServerID, rec.Outcome, rec.Code, rec.Detail, rec.DurationMs, rec.Truncated, info, tools,
		started, float64(intervalSecs), iconData, iconType)
	return err
}

func (s *CheckRepository) Release(ctx context.Context, id uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE mcp_server_checks
		SET lease_until = NULL
		WHERE server_id = $1`, id)
	return err
}

func (s *CheckRepository) Requeue(ctx context.Context, id uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE mcp_server_checks
		SET next_check_at = now()
		WHERE server_id = $1`, id)
	return err
}

func (s *CheckRepository) MetricCounts(ctx context.Context) (map[string]int64, error) {
	return postgres.Pairs[string, int64](ctx, s.db.From(ctx), constraints, `
		SELECT 'servers:' || CASE
				WHEN s.status = 'disabled' THEN 'disabled'
				WHEN c.outcome = 'ok' THEN 'ok'
				WHEN c.outcome = 'failed' AND c.tools IS NOT NULL THEN 'failing'
				ELSE 'unknown'
			END, count(*)
		FROM mcp_servers s
		LEFT JOIN mcp_server_checks c ON c.server_id = s.id
		GROUP BY 1
		UNION ALL
		SELECT 'info:' || id::text || '/' || slug, 1
		FROM mcp_servers`)
}

func (s *CheckRepository) Icon(ctx context.Context, server uuid.UUID) (*gatewaytypes.IconFile, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (gatewaytypes.IconFile, error) {
		var f gatewaytypes.IconFile
		err := r.Scan(&f.Data, &f.Type)
		return f, err
	}, `
		SELECT icon, icon_type
		FROM mcp_server_checks
		WHERE server_id = $1
			AND icon IS NOT NULL`, server)
}
