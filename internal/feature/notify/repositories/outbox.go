package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type OutboxRepository struct{ db *pgtx.DB }

func NewOutboxRepository(db *pgtx.DB) *OutboxRepository { return &OutboxRepository{db: db} }

const pruneBatch = 5000

func (s *OutboxRepository) InsertEvent(ctx context.Context, ev notifytypes.NewEvent) (bool, error) {
	inserted, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		INSERT INTO notification_events (id, type, node_id, payload, dedupe_key, occurred_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6)
		ON CONFLICT (dedupe_key)
		DO NOTHING
		RETURNING id`, ev.ID, string(ev.Type), ev.NodeID, string(ev.Payload), ev.DedupeKey, ev.OccurredAt)
	return inserted != nil, err
}

func (s *OutboxRepository) InsertDeliveries(ctx context.Context, eventID uuid.UUID, ids, subscriptions []uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO notification_deliveries (id, event_id, subscription_id)
		SELECT d, $1, s
		FROM unnest($2::uuid[], $3::uuid[]) AS x(d, s)`, eventID, ids, subscriptions)
	return err
}

func (s *OutboxRepository) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		UPDATE notification_deliveries
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id IN (
			SELECT id
			FROM notification_deliveries
			WHERE status = 'pending'
				AND next_run_at <= now()
				AND (lease_until IS NULL OR lease_until < now())
			ORDER BY next_run_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		RETURNING id`, limit, leaseSecs)
}

func (s *OutboxRepository) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE notification_deliveries
		SET lease_until = now() + make_interval(secs => $2)
		WHERE id = $1
			AND status = 'pending'`, id, leaseSecs)
	return err
}

func (s *OutboxRepository) Job(ctx context.Context, id uuid.UUID) (*notifytypes.Job, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanJob, `
		SELECT d.id, d.subscription_id, d.status, d.attempts, e.payload
		FROM notification_deliveries d
		JOIN notification_events e ON e.id = d.event_id
		WHERE d.id = $1`, id)
}

func (s *OutboxRepository) Finish(ctx context.Context, id uuid.UUID, o notifytypes.Outcome) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE notification_deliveries
		SET status = $2, attempts = attempts + 1, next_run_at = now() + make_interval(secs => $3), lease_until = NULL,
			last_http_status = $4, last_error_code = $5, last_error_message = $6,
			delivered_at = CASE WHEN $2 = 'delivered' THEN now() END, updated_at = now()
		WHERE id = $1`, id, string(o.Status), o.NextAttemptIn.Seconds(), o.HTTPStatus, o.ErrorCode, o.ErrorMessage)
	return err
}

func (s *OutboxRepository) Journal(ctx context.Context, nodeID uuid.UUID, f notifytypes.DeliveryFilter, page accesstypes.PageRequest) (accesstypes.Page[notifytypes.Delivery], error) {
	q := s.db.From(ctx)
	total, err := postgres.One(ctx, q, constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM notification_deliveries d
		JOIN notification_subscriptions s ON s.id = d.subscription_id
		WHERE s.node_id = $1
			AND ($2::text IS NULL OR s.key = $2)
			AND ($3::text IS NULL OR d.status = $3)`, nodeID, f.Subscription, f.Status)
	if err != nil {
		return accesstypes.Page[notifytypes.Delivery]{}, err
	}
	items, err := postgres.List(ctx, q, constraints, scanDelivery, `
		SELECT d.id, d.event_id, e.type, e.node_id, e.payload -> 'node' ->> 'path', s.key, s.channel, d.status,
			d.attempts, CASE WHEN d.status = 'pending' THEN rfc3339(d.next_run_at) END, d.last_http_status,
			d.last_error_code, d.last_error_message, rfc3339(d.created_at), rfc3339(d.delivered_at)
		FROM notification_deliveries d
		JOIN notification_subscriptions s ON s.id = d.subscription_id
		JOIN notification_events e ON e.id = d.event_id
		WHERE s.node_id = $1
			AND ($2::text IS NULL OR s.key = $2)
			AND ($3::text IS NULL OR d.status = $3)
		ORDER BY d.created_at DESC, d.id DESC
		LIMIT $4
		OFFSET $5`, nodeID, f.Subscription, f.Status, int64(page.Limit), int64(page.Offset))
	return accesstypes.Page[notifytypes.Delivery]{Items: items, Total: uint64(max(total, 0)), Limit: page.Limit, Offset: page.Offset}, err
}

func (s *OutboxRepository) Prune(ctx context.Context, retentionDays uint32) (int64, error) {
	var deleted int64
	for {
		n, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
			DELETE FROM notification_events
			WHERE id IN (
				SELECT id
				FROM notification_events
				WHERE created_at < now() - make_interval(days => $1::int)
				LIMIT $2
			)`, int(retentionDays), int64(pruneBatch))
		if err != nil {
			return deleted, err
		}
		deleted += n
		if n < pruneBatch {
			return deleted, nil
		}
	}
}

func (s *OutboxRepository) QueueCounts(ctx context.Context) (map[string]int64, error) {
	return postgres.Pairs[string, int64](ctx, s.db.From(ctx), constraints, `
		SELECT 'queue:notify_deliveries/' || CASE WHEN lease_until > now() THEN 'leased' ELSE 'due' END, count(*)
		FROM notification_deliveries
		WHERE status = 'pending'
			AND (lease_until > now() OR next_run_at <= now())
		GROUP BY 1`)
}
