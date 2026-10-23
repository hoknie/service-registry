package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/deploy/repositories"
	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/pkg/secretbox"
)

func (s *PortsService) ActivitySignals(ctx context.Context, projects []uuid.UUID, _, busy bool) (map[uuid.UUID][]catalogtypes.ProcessSignals, error) {
	return s.clusterRepository.Signals(ctx, projects, busy)
}

func (s *PortsService) AdoptLegacyCredentials(ctx context.Context, adopt catalogtypes.AdoptFunc) (int, error) {
	rows, err := s.clusterRepository.Legacy(ctx)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		value := catalogtypes.SecretValue{Ref: row.Ref}
		if row.Ref == nil {
			v, err := s.secrets.Open(*row.Enc, secretbox.AAD(deploytypes.Table, row.ID.String(), deploytypes.ColumnCredentials))
			if err != nil {
				return 0, fmt.Errorf("cluster %s: %w", row.ID, err)
			}
			value.Value = &v
		}
		secret, err := adopt(ctx, catalogtypes.LegacyCredential{RecordKind: "cluster", RecordID: row.ID, Name: row.Name, Value: value})
		if err != nil {
			return 0, fmt.Errorf("cluster %s: %w", row.ID, err)
		}
		if err := s.clusterRepository.AdoptSecret(ctx, row.ID, secret); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

func (s *PortsService) LegacyCredentialCount(ctx context.Context) (int, error) {
	rows, err := s.clusterRepository.Legacy(ctx)
	return len(rows), err
}

func (s *PortsService) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return s.clusterRepository.SecretUsage(ctx, secrets)
}

type PortsService struct {
	clusterRepository *repositories.ClusterRepository
	secrets           *secretbox.Box
}
