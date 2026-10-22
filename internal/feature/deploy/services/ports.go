package services

import (
	"context"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/deploy/repositories"
	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/pkg/secretbox"
)

func (s *PortsService) ActivitySignals(ctx context.Context, projects []uuid.UUID, _, busy bool) (map[uuid.UUID][]catalogtypes.ProcessSignals, error) {
	return s.clusterRepository.Signals(ctx, projects, busy)
}

func (s *PortsService) PrepareSecretRotation(ctx context.Context) (func(context.Context) error, int, error) {
	clusters, err := s.clusterRepository.Secrets(ctx)
	if err != nil {
		return nil, 0, err
	}
	plain := make([]string, len(clusters))
	for i, c := range clusters {
		v, err := s.secrets.Open(c.Enc, secretbox.AAD(deploytypes.Table, c.ID.String(), deploytypes.ColumnCredentials))
		if err != nil {
			return nil, 0, err
		}
		plain[i] = v
	}
	apply := func(ctx context.Context) error {
		for i, c := range clusters {
			enc, err := s.secrets.Seal(plain[i], secretbox.AAD(deploytypes.Table, c.ID.String(), deploytypes.ColumnCredentials))
			if err != nil {
				return err
			}
			if err := s.clusterRepository.ReplaceSecret(ctx, c.ID, enc); err != nil {
				return err
			}
		}
		return nil
	}
	return apply, len(clusters), nil
}

func (s *PortsService) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return s.clusterRepository.SecretUsage(ctx, secrets)
}

type PortsService struct {
	clusterRepository *repositories.ClusterRepository
	secrets           *secretbox.Box
}
