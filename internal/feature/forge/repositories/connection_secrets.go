package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/postgres"
)

func (s *ConnectionRepository) AllSecrets(ctx context.Context) ([]forgetypes.SecretRow, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT id, credentials_enc, webhook_secret_enc
		FROM forge_connections
		ORDER BY id`)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (forgetypes.SecretRow, error) {
		var row forgetypes.SecretRow
		err := r.Scan(&row.ID, &row.CredentialsEnc, &row.WebhookSecretEnc)
		return row, err
	})
	return out, postgres.Translate(err, constraints)
}

func (s *ConnectionRepository) ReplaceSecrets(ctx context.Context, row forgetypes.SecretRow) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE forge_connections
		SET credentials_enc = $2, webhook_secret_enc = $3
		WHERE id = $1`, row.ID, row.CredentialsEnc, row.WebhookSecretEnc)
	return postgres.Translate(err, constraints)
}

func (s *ConnectionRepository) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT credentials_secret_id, count(*)
		FROM forge_connections
		WHERE credentials_secret_id = ANY($1)
		GROUP BY credentials_secret_id`, secrets)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	out := map[uuid.UUID]int64{}
	var id uuid.UUID
	var n int64
	_, err = pgx.ForEachRow(rows, []any{&id, &n}, func() error {
		out[id] = n
		return nil
	})
	return out, postgres.Translate(err, constraints)
}
