package services

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/notify/repositories"
	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/pkg/secretbox"
)

func (s *PortsService) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return s.subscriptionRepository.SecretUsage(ctx, secrets)
}

func (s *PortsService) PrepareSecretRotation(ctx context.Context) (func(context.Context) error, int, error) {
	rows, err := s.subscriptionRepository.Signing(ctx)
	if err != nil {
		return nil, 0, err
	}
	plain := make([]string, len(rows))
	for i, row := range rows {
		v, err := s.secrets.Open(row.Enc, secretbox.AAD(notifytypes.Table, row.ID.String(), notifytypes.ColumnSigning))
		if err != nil {
			return nil, 0, err
		}
		plain[i] = v
	}
	apply := func(ctx context.Context) error {
		for i, row := range rows {
			enc, err := s.secrets.Seal(plain[i], secretbox.AAD(notifytypes.Table, row.ID.String(), notifytypes.ColumnSigning))
			if err != nil {
				return err
			}
			if err := s.subscriptionRepository.ReplaceSigning(ctx, notifytypes.SigningRow{ID: row.ID, Enc: enc}); err != nil {
				return err
			}
		}
		return nil
	}
	return apply, len(rows), nil
}

type PortsService struct {
	secrets                *secretbox.Box
	subscriptionRepository *repositories.SubscriptionRepository
}
