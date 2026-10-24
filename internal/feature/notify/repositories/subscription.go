package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type SubscriptionRepository struct{ db *pgtx.DB }

func NewSubscriptionRepository(db *pgtx.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (s *SubscriptionRepository) Effective(ctx context.Context, nodeID uuid.UUID) ([]notifytypes.Subscription, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, scanSubscription, `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, 0 AS lvl
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT p.id, p.parent_id, up.lvl + 1
			FROM nodes p
			JOIN up ON p.id = up.parent_id
		)
		SELECT DISTINCT ON (s.key) s.id, s.node_id, s.key, s.channel, s.secret_id, sec.name, s.signing_secret_enc,
			s.signing_fingerprint, s.event_types, s.environments, s.locale, s.disabled, rfc3339(s.created_at),
			rfc3339(s.updated_at), up.lvl > 0 AS inherited
		FROM up
		JOIN notification_subscriptions s ON s.node_id = up.id
		LEFT JOIN secrets sec ON sec.id = s.secret_id
		ORDER BY s.key, up.lvl`, nodeID)
}

func (s *SubscriptionRepository) Get(ctx context.Context, id uuid.UUID) (*notifytypes.Subscription, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanSubscription, `
		SELECT s.id, s.node_id, s.key, s.channel, s.secret_id, sec.name, s.signing_secret_enc, s.signing_fingerprint,
			s.event_types, s.environments, s.locale, s.disabled, rfc3339(s.created_at), rfc3339(s.updated_at),
			false AS inherited
		FROM notification_subscriptions s
		LEFT JOIN secrets sec ON sec.id = s.secret_id
		WHERE s.id = $1`, id)
}

func (s *SubscriptionRepository) LockNode(ctx context.Context, nodeID uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		SELECT 1
		FROM nodes
		WHERE id = $1 FOR NO KEY UPDATE`, nodeID)
}

func (s *SubscriptionRepository) CountOthers(ctx context.Context, nodeID uuid.UUID, key string) (int64, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM notification_subscriptions
		WHERE node_id = $1
			AND key <> $2`, nodeID, key)
}

func (s *SubscriptionRepository) Upsert(ctx context.Context, id, nodeID uuid.UUID, n notifytypes.NewSubscription) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO notification_subscriptions AS s (id, node_id, key, channel, secret_id, event_types, environments,
			locale, disabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (node_id, key)
		DO UPDATE SET channel = EXCLUDED.channel, secret_id = EXCLUDED.secret_id, event_types = EXCLUDED.event_types,
			environments = EXCLUDED.environments, locale = EXCLUDED.locale, disabled = EXCLUDED.disabled,
			signing_secret_enc = CASE WHEN EXCLUDED.channel = 'webhook' THEN s.signing_secret_enc END,
			signing_fingerprint = CASE WHEN EXCLUDED.channel = 'webhook' THEN s.signing_fingerprint END,
			updated_at = now()`, id, nodeID, n.Key, string(n.Channel), n.SecretID,
		eventTypes(n.EventTypes), n.Environments, n.Locale, n.Disabled)
	return err
}

func (s *SubscriptionRepository) Delete(ctx context.Context, nodeID uuid.UUID, key string) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM notification_subscriptions
		WHERE node_id = $1
			AND key = $2`, nodeID, key)
}

func (s *SubscriptionRepository) SetSigning(ctx context.Context, id uuid.UUID, enc, fingerprint string) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE notification_subscriptions
		SET signing_secret_enc = $2, signing_fingerprint = $3, updated_at = now()
		WHERE id = $1`, id, enc, fingerprint)
}

func (s *SubscriptionRepository) MarkTest(ctx context.Context, id uuid.UUID, everySecs int) (bool, error) {
	marked, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		UPDATE notification_subscriptions
		SET last_test_at = now()
		WHERE id = $1
			AND (last_test_at IS NULL OR last_test_at <= now() - make_interval(secs => $2))
		RETURNING true`, id, everySecs)
	return marked != nil, err
}

func (s *SubscriptionRepository) TestWait(ctx context.Context, id uuid.UUID, everySecs int) (int64, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT ceil(extract(epoch FROM last_test_at + make_interval(secs => $2) - now()))::bigint
		FROM notification_subscriptions
		WHERE id = $1`, id, everySecs)
}

func (s *SubscriptionRepository) Signing(ctx context.Context) ([]notifytypes.SigningRow, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (notifytypes.SigningRow, error) {
		var row notifytypes.SigningRow
		err := r.Scan(&row.ID, &row.Enc)
		return row, err
	}, `
		SELECT id, signing_secret_enc
		FROM notification_subscriptions
		WHERE signing_secret_enc IS NOT NULL
		ORDER BY id`)
}

func (s *SubscriptionRepository) ReplaceSigning(ctx context.Context, row notifytypes.SigningRow) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE notification_subscriptions
		SET signing_secret_enc = $2
		WHERE id = $1`, row.ID, row.Enc)
	return err
}

func (s *SubscriptionRepository) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return postgres.Pairs[uuid.UUID, int64](ctx, s.db.From(ctx), constraints, `
		SELECT secret_id, count(*)
		FROM notification_subscriptions
		WHERE secret_id = ANY($1)
		GROUP BY secret_id`, secrets)
}
