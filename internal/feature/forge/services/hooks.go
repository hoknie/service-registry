package services

import (
	"context"

	"github.com/google/uuid"

	forgeports "svc-registry/internal/feature/forge/ports"
	"svc-registry/internal/feature/forge/repositories"
	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/secretbox"
)

func (s *HooksService) AcceptDelivery(ctx context.Context, rawID string, header func(string) string, body []byte) error {
	refused := apperr.New(apperr.InvalidWebhookSignature)
	id, err := uuid.Parse(rawID)
	if err != nil {
		return refused
	}
	conn, err := s.connectionRepository.Find(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	if conn == nil {
		return refused
	}
	sec, err := s.connectionRepository.Secrets(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	if sec == nil || sec.WebhookSecretEnc == nil {
		return refused
	}
	secret, err := s.secrets.Open(*sec.WebhookSecretEnc, secretbox.AAD(forgetypes.Table, id.String(), forgetypes.ColumnWebhookSecret))
	if err != nil || !s.clients.VerifyDelivery(conn.Kind, secret, header, body) {
		return refused
	}
	return apperr.Wrap(s.connectionRepository.ScheduleNow(ctx, id, forgetypes.TriggerWebhook))
}

type HooksService struct {
	clients              forgeports.ClientFactoryPort
	connectionRepository *repositories.ConnectionRepository
	secrets              *secretbox.Box
	connectionsService   *ConnectionsService
	sync                 *SyncService
}
