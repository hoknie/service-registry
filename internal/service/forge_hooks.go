package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/deploy"
	"svc-registry/internal/forge"
	"svc-registry/internal/knowledge"
	"svc-registry/pkg/secretbox"
)

func AcceptDelivery(ctx context.Context, state *State, rawID string, header func(string) string, body []byte) error {
	refused := apperr.New(apperr.InvalidWebhookSignature)
	id, err := uuid.Parse(rawID)
	if err != nil {
		return refused
	}
	conn, err := state.Connections.Find(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	if conn == nil {
		return refused
	}
	sec, err := state.Connections.Secrets(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	if sec == nil || sec.WebhookSecretEnc == nil {
		return refused
	}
	secret, err := state.Secrets.Open(*sec.WebhookSecretEnc, secretbox.AAD(forge.Table, id.String(), forge.ColumnWebhookSecret))
	if err != nil || !state.Forges.VerifyDelivery(conn.Kind, secret, header, body) {
		return refused
	}
	return apperr.Wrap(state.Connections.ScheduleNow(ctx, id, forge.TriggerWebhook))
}

func RotateSecrets(ctx context.Context, state *State) (int, error) {
	if !state.Secrets.CanEncrypt() {
		return 0, forge.ConflictSecretsKeyMissing
	}
	rows, err := state.Connections.AllSecrets(ctx)
	if err != nil {
		return 0, err
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
			v, err := state.Secrets.Open(*c.value, secretbox.AAD(forge.Table, row.ID.String(), c.name))
			if err != nil {
				return 0, err
			}
			plain[i][j] = &v
			n++
		}
	}
	clusters, err := state.Clusters.Secrets(ctx)
	if err != nil {
		return 0, err
	}
	clusterPlain := make([]string, len(clusters))
	for i, c := range clusters {
		v, err := state.Secrets.Open(c.Enc, secretbox.AAD(deploy.Table, c.ID.String(), deploy.ColumnCredentials))
		if err != nil {
			return 0, err
		}
		clusterPlain[i] = v
		n++
	}
	sources, err := state.Sources.Secrets(ctx)
	if err != nil {
		return 0, err
	}
	sourcePlain := make([]string, len(sources))
	for i, s := range sources {
		v, err := state.Secrets.Open(s.CredentialsEnc, secretbox.AAD(knowledge.SourceTable, s.ProjectID.String(), knowledge.SourceColumnCredentials))
		if err != nil {
			return 0, err
		}
		sourcePlain[i] = v
		n++
	}
	for i, row := range rows {
		next := forge.SecretRow{ID: row.ID}
		for j, name := range []string{forge.ColumnCredentials, forge.ColumnWebhookSecret} {
			if plain[i][j] == nil {
				continue
			}
			enc, err := state.Secrets.Seal(*plain[i][j], secretbox.AAD(forge.Table, row.ID.String(), name))
			if err != nil {
				return 0, err
			}
			if j == 0 {
				next.CredentialsEnc = &enc
			} else {
				next.WebhookSecretEnc = &enc
			}
		}
		if err := state.Connections.ReplaceSecrets(ctx, next); err != nil {
			return 0, err
		}
	}
	for i, c := range clusters {
		enc, err := state.Secrets.Seal(clusterPlain[i], secretbox.AAD(deploy.Table, c.ID.String(), deploy.ColumnCredentials))
		if err != nil {
			return 0, err
		}
		if err := state.Clusters.ReplaceSecret(ctx, c.ID, enc); err != nil {
			return 0, err
		}
	}
	for i, s := range sources {
		enc, err := state.Secrets.Seal(sourcePlain[i], secretbox.AAD(knowledge.SourceTable, s.ProjectID.String(), knowledge.SourceColumnCredentials))
		if err != nil {
			return 0, err
		}
		if err := state.Sources.ReplaceSecret(ctx, s.ProjectID, enc); err != nil {
			return 0, err
		}
	}
	return n, nil
}
