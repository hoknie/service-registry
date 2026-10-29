package services

import (
	"context"

	"github.com/google/uuid"

	reposyncports "svc-registry/internal/feature/reposync/ports"
	"svc-registry/internal/feature/reposync/repositories"
	reposynctypes "svc-registry/internal/feature/reposync/types"
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
	secret, err := s.secrets.Open(*sec.WebhookSecretEnc, secretbox.AAD(reposynctypes.LegacyTable, id.String(), reposynctypes.ColumnWebhookSecret))
	if err != nil || !s.clients.VerifyDelivery(conn.Kind, secret, header, body) {
		return refused
	}
	if s.sync.effective(ctx, conn.NodeID).Off() {
		return nil
	}
	return apperr.Wrap(s.connectionRepository.ScheduleNow(ctx, id, reposynctypes.TriggerWebhook))
}

type HooksService struct {
	clients              reposyncports.ClientFactoryPort
	connectionRepository *repositories.ConnectionRepository
	secrets              *secretbox.Box
	connectionsService   *ConnectionsService
	sync                 *SyncService
}
