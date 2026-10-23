package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/forge/repositories"
	forgetypes "svc-registry/internal/feature/forge/types"
	"svc-registry/pkg/secretbox"
)

func (s *PortsService) Managed(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	return s.repoRepository.Managed(ctx, ids)
}

func (s *PortsService) ActivitySignals(ctx context.Context, projects []uuid.UUID, _, busy bool) (map[uuid.UUID][]catalogtypes.ProcessSignals, error) {
	return s.connectionRepository.Signals(ctx, projects, busy)
}

func (s *PortsService) ContainerSignals(ctx context.Context, nodeID uuid.UUID) ([]catalogtypes.ProcessSignals, error) {
	return s.connectionRepository.ContainerSignals(ctx, nodeID)
}

func (s *PortsService) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return s.connectionRepository.SecretUsage(ctx, secrets)
}

func (s *PortsService) PrepareSecretRotation(ctx context.Context) (func(context.Context) error, int, error) {
	rows, err := s.connectionRepository.WebhookSecrets(ctx)
	if err != nil {
		return nil, 0, err
	}
	plain := make([]string, len(rows))
	for i, row := range rows {
		v, err := s.secrets.Open(row.WebhookSecretEnc, secretbox.AAD(forgetypes.Table, row.ID.String(), forgetypes.ColumnWebhookSecret))
		if err != nil {
			return nil, 0, err
		}
		plain[i] = v
	}
	apply := func(ctx context.Context) error {
		for i, row := range rows {
			enc, err := s.secrets.Seal(plain[i], secretbox.AAD(forgetypes.Table, row.ID.String(), forgetypes.ColumnWebhookSecret))
			if err != nil {
				return err
			}
			if err := s.connectionRepository.ReplaceWebhookSecret(ctx, forgetypes.SecretRow{ID: row.ID, WebhookSecretEnc: enc}); err != nil {
				return err
			}
		}
		return nil
	}
	return apply, len(rows), nil
}

func (s *PortsService) AdoptLegacyCredentials(ctx context.Context, adopt catalogtypes.AdoptFunc) (int, error) {
	rows, err := s.connectionRepository.Legacy(ctx)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		value := catalogtypes.SecretValue{Ref: row.Ref}
		if row.Ref == nil {
			v, err := s.secrets.Open(*row.Enc, secretbox.AAD(forgetypes.Table, row.ID.String(), forgetypes.ColumnCredentials))
			if err != nil {
				return 0, fmt.Errorf("forge connection %s: %w", row.ID, err)
			}
			value.Value = &v
		}
		secret, err := adopt(ctx, catalogtypes.LegacyCredential{RecordKind: "forge", RecordID: row.ID, Scope: &row.NodeID,
			Name: row.OwnerPath, Value: value})
		if err != nil {
			return 0, fmt.Errorf("forge connection %s: %w", row.ID, err)
		}
		if err := s.connectionRepository.AdoptSecret(ctx, row.ID, secret); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

func (s *PortsService) LegacyCredentialCount(ctx context.Context) (int, error) {
	rows, err := s.connectionRepository.Legacy(ctx)
	return len(rows), err
}

type PortsService struct {
	connectionRepository *repositories.ConnectionRepository
	repoRepository       *repositories.RepoRepository
	secrets              *secretbox.Box
}
