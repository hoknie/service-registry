package services

import (
	"context"

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
	rows, err := s.connectionRepository.AllSecrets(ctx)
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
		for j, c := range []column{{row.CredentialsEnc, forgetypes.ColumnCredentials}, {row.WebhookSecretEnc, forgetypes.ColumnWebhookSecret}} {
			if c.value == nil {
				continue
			}
			v, err := s.secrets.Open(*c.value, secretbox.AAD(forgetypes.Table, row.ID.String(), c.name))
			if err != nil {
				return nil, 0, err
			}
			plain[i][j] = &v
			n++
		}
	}
	apply := func(ctx context.Context) error {
		for i, row := range rows {
			next := forgetypes.SecretRow{ID: row.ID}
			for j, name := range []string{forgetypes.ColumnCredentials, forgetypes.ColumnWebhookSecret} {
				if plain[i][j] == nil {
					continue
				}
				enc, err := s.secrets.Seal(*plain[i][j], secretbox.AAD(forgetypes.Table, row.ID.String(), name))
				if err != nil {
					return err
				}
				if j == 0 {
					next.CredentialsEnc = &enc
				} else {
					next.WebhookSecretEnc = &enc
				}
			}
			if err := s.connectionRepository.ReplaceSecrets(ctx, next); err != nil {
				return err
			}
		}
		return nil
	}
	return apply, n, nil
}

type PortsService struct {
	connectionRepository *repositories.ConnectionRepository
	repoRepository       *repositories.RepoRepository
	secrets              *secretbox.Box
}
