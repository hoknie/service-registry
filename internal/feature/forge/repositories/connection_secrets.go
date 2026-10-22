package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/postgres"
)

func (s *ConnectionRepository) AllSecrets(ctx context.Context) ([]forgetypes.SecretRow, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (forgetypes.SecretRow, error) {
		var row forgetypes.SecretRow
		err := r.Scan(&row.ID, &row.CredentialsEnc, &row.WebhookSecretEnc)
		return row, err
	}, `
		SELECT id, credentials_enc, webhook_secret_enc
		FROM forge_connections
		ORDER BY id`)
}

func (s *ConnectionRepository) ReplaceSecrets(ctx context.Context, row forgetypes.SecretRow) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE forge_connections
		SET credentials_enc = $2, webhook_secret_enc = $3
		WHERE id = $1`, row.ID, row.CredentialsEnc, row.WebhookSecretEnc)
	return err
}

func (s *ConnectionRepository) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return postgres.Pairs[uuid.UUID, int64](ctx, s.db.From(ctx), constraints, `
		SELECT credentials_secret_id, count(*)
		FROM forge_connections
		WHERE credentials_secret_id = ANY($1)
		GROUP BY credentials_secret_id`, secrets)
}
