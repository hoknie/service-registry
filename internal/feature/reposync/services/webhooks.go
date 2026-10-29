package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	reposyncports "svc-registry/internal/feature/reposync/ports"
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/secretbox"
)

type WebhookSetup struct {
	Mode   reposynctypes.WebhookMode
	URL    string
	Secret *string
}

func (s *HooksService) SetupWebhook(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID, rawMode string) (WebhookSetup, error) {
	c, err := s.connectionsService.authorizeConnection(ctx, p, catalogtypes.PermAccess, nodeID, id)
	if err != nil {
		return WebhookSetup{}, err
	}
	mode, err := reposynctypes.ValidateWebhookMode(rawMode)
	if err != nil {
		return WebhookSetup{}, apperr.Wrap(err)
	}
	url := s.connectionsService.webhookURL(id)
	if url == "" {
		return WebhookSetup{}, apperr.Wrap(reposynctypes.ConflictPublicURLMissing)
	}
	if !s.secrets.CanEncrypt() {
		return WebhookSetup{}, apperr.Wrap(reposynctypes.ConflictSecretsKeyMissing)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return WebhookSetup{}, apperr.Internalf("no randomness: %v", err)
	}
	secret := hex.EncodeToString(raw)
	enc, err := s.secrets.Seal(secret, secretbox.AAD(reposynctypes.LegacyTable, id.String(), reposynctypes.ColumnWebhookSecret))
	if err != nil {
		return WebhookSetup{}, apperr.Internalf("encrypt: %v", err)
	}
	prev, err := s.connectionRepository.Secrets(ctx, id)
	if err != nil {
		return WebhookSetup{}, apperr.Wrap(err)
	}
	var client reposyncports.ClientPort
	needClient := mode == reposynctypes.WebhookRegister || (prev != nil && prev.WebhookID != nil)
	if needClient {
		if client, err = s.sync.clientFor(ctx, c, s.sync.effective(ctx, c.NodeID).Timeout()); err != nil {
			return WebhookSetup{}, apperr.Wrap(err)
		}
	}
	var hookID *string
	if mode == reposynctypes.WebhookRegister {
		created, err := client.RegisterHook(ctx, url, secret)
		if err != nil {
			return WebhookSetup{}, apperr.Wrap(err)
		}
		hookID = &created
	}
	if prev != nil && prev.WebhookID != nil {
		if err := client.DeleteHook(ctx, *prev.WebhookID); err != nil {
			slog.Warn("previous hosting webhook not removed", "connection", id, "error", err)
		}
	}
	if err := s.connectionRepository.SetWebhook(ctx, id, reposynctypes.Webhook{Mode: &mode, SecretEnc: &enc, HookID: hookID}); err != nil {
		return WebhookSetup{}, apperr.Wrap(err)
	}
	out := WebhookSetup{Mode: mode, URL: url}
	if mode == reposynctypes.WebhookManual {
		out.Secret = &secret
	}
	return out, nil
}

func (s *HooksService) DeleteWebhook(ctx context.Context, p accesstypes.Principal, nodeID, id uuid.UUID) error {
	c, err := s.connectionsService.authorizeConnection(ctx, p, catalogtypes.PermAccess, nodeID, id)
	if err != nil {
		return err
	}
	sec, err := s.connectionRepository.Secrets(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	if sec != nil && sec.WebhookID != nil {
		client, err := s.sync.clientFor(ctx, c, s.sync.effective(ctx, c.NodeID).Timeout())
		if err != nil {
			return apperr.Wrap(err)
		}
		if err := client.DeleteHook(ctx, *sec.WebhookID); err != nil {
			return apperr.Wrap(err)
		}
	}
	return apperr.Wrap(s.connectionRepository.SetWebhook(ctx, id, reposynctypes.Webhook{}))
}
