package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/forge"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/secretbox"
)

func (s *Service) AcceptDelivery(ctx context.Context, rawID string, header func(string) string, body []byte) error {
	refused := apperr.New(apperr.InvalidWebhookSignature)
	id, err := uuid.Parse(rawID)
	if err != nil {
		return refused
	}
	conn, err := s.connections.Find(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	if conn == nil {
		return refused
	}
	sec, err := s.connections.Secrets(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	if sec == nil || sec.WebhookSecretEnc == nil {
		return refused
	}
	secret, err := s.secrets.Open(*sec.WebhookSecretEnc, secretbox.AAD(forge.Table, id.String(), forge.ColumnWebhookSecret))
	if err != nil || !s.clients.VerifyDelivery(conn.Kind, secret, header, body) {
		return refused
	}
	return apperr.Wrap(s.connections.ScheduleNow(ctx, id, forge.TriggerWebhook))
}

func (s *Service) PrepareSecretRotation(ctx context.Context) (func(context.Context) error, int, error) {
	rows, err := s.connections.AllSecrets(ctx)
	if err != nil {
		return nil, 0, err
	}
	type column struct {
		value *string
		name  string
	}
	plain := make([][2]*string, len(rows))
	n := 0
	for i, row := range rows {
		for j, c := range []column{{row.CredentialsEnc, forge.ColumnCredentials}, {row.WebhookSecretEnc, forge.ColumnWebhookSecret}} {
			if c.value == nil {
				continue
			}
			v, err := s.secrets.Open(*c.value, secretbox.AAD(forge.Table, row.ID.String(), c.name))
			if err != nil {
				return nil, 0, err
			}
			plain[i][j] = &v
			n++
		}
	}
	apply := func(ctx context.Context) error {
		for i, row := range rows {
			next := forge.SecretRow{ID: row.ID}
			for j, name := range []string{forge.ColumnCredentials, forge.ColumnWebhookSecret} {
				if plain[i][j] == nil {
					continue
				}
				enc, err := s.secrets.Seal(*plain[i][j], secretbox.AAD(forge.Table, row.ID.String(), name))
				if err != nil {
					return err
				}
				if j == 0 {
					next.CredentialsEnc = &enc
				} else {
					next.WebhookSecretEnc = &enc
				}
			}
			if err := s.connections.ReplaceSecrets(ctx, next); err != nil {
				return err
			}
		}
		return nil
	}
	return apply, n, nil
}
