package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/postgres"
)

func (s *ConnectionRepository) WebhookSecrets(ctx context.Context) ([]forgetypes.SecretRow, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (forgetypes.SecretRow, error) {
		var row forgetypes.SecretRow
		err := r.Scan(&row.ID, &row.WebhookSecretEnc)
		return row, err
	}, `
		SELECT id, webhook_secret_enc
		FROM forge_connections
		WHERE webhook_secret_enc IS NOT NULL
		ORDER BY id`)
}

func (s *ConnectionRepository) ReplaceWebhookSecret(ctx context.Context, row forgetypes.SecretRow) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE forge_connections
		SET webhook_secret_enc = $2
		WHERE id = $1`, row.ID, row.WebhookSecretEnc)
	return err
}

func (s *ConnectionRepository) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return postgres.Pairs[uuid.UUID, int64](ctx, s.db.From(ctx), constraints, `
		SELECT credentials_secret_id, count(*)
		FROM forge_connections
		WHERE credentials_secret_id = ANY($1)
		GROUP BY credentials_secret_id`, secrets)
}

func (s *ConnectionRepository) Legacy(ctx context.Context) ([]forgetypes.LegacyConnection, error) {
	tx := s.db.From(ctx)
	if ok, err := postgres.HasColumn(ctx, tx, forgetypes.Table, forgetypes.ColumnCredentials); err != nil || !ok {
		return nil, err
	}
	return postgres.List(ctx, tx, constraints, func(r pgx.CollectableRow) (forgetypes.LegacyConnection, error) {
		var c forgetypes.LegacyConnection
		err := r.Scan(&c.ID, &c.NodeID, &c.OwnerPath, &c.Enc, &c.Ref)
		return c, err
	}, `
		SELECT id, node_id, owner_path, credentials_enc, credentials_ref
		FROM forge_connections
		WHERE credentials_secret_id IS NULL
			AND (credentials_enc IS NOT NULL OR credentials_ref IS NOT NULL)
		ORDER BY id`)
}

func (s *ConnectionRepository) AdoptSecret(ctx context.Context, id, secret uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE forge_connections
		SET credentials_secret_id = $2, credentials_enc = NULL, credentials_ref = NULL, credentials_fingerprint = NULL,
			updated_at = now()
		WHERE id = $1`, id, secret)
}
