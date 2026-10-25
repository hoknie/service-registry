package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/deploy/repositories"
	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/metrics"
	"svc-registry/pkg/secretbox"
)

func (s *PortsService) ActivitySignals(ctx context.Context, projects []uuid.UUID, _, busy bool) (map[uuid.UUID][]catalogtypes.ProcessSignals, error) {
	return s.clusterRepository.Signals(ctx, projects, busy)
}

func (s *PortsService) legacy(ctx context.Context) ([]deploytypes.LegacyCluster, error) {
	ok, err := s.clusterRepository.HasLegacyColumn(ctx)
	if err != nil || !ok {
		return nil, err
	}
	return s.clusterRepository.Legacy(ctx)
}

func (s *PortsService) AdoptLegacyCredentials(ctx context.Context, adopt catalogtypes.AdoptFunc) (int, error) {
	rows, err := s.legacy(ctx)
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
	rows, err := s.legacy(ctx)
	return len(rows), err
}

func (s *PortsService) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return s.clusterRepository.SecretUsage(ctx, secrets)
}

type PortsService struct {
	clusterRepository *repositories.ClusterRepository
	secrets           *secretbox.Box
}

var metricShapes = map[string]metrics.Shape{
	"clusters":    {Name: "svcr_clusters", Labels: []string{"mode", "state"}},
	"workloads":   {Name: "svcr_cluster_workloads"},
	"credentials": {Name: "svcr_credentials_active", Labels: []string{"kind"}},
	"drifts":      {Name: "svcr_drifts"},
	"queue":       {Name: "svcr_job_queue", Labels: []string{"job", "state"}},
}

func (s *PortsService) MetricSamples(ctx context.Context) ([]metrics.Sample, error) {
	counts, err := s.clusterRepository.MetricCounts(ctx)
	if err != nil {
		return nil, err
	}
	return metrics.FromPairs(counts, metricShapes), nil
}

func (s *PortsService) QueueSamples(ctx context.Context) ([]metrics.Sample, error) {
	counts, err := s.clusterRepository.QueueCounts(ctx)
	if err != nil {
		return nil, err
	}
	return metrics.FromPairs(metrics.Zero(counts, "queue:cluster_polls/due", "queue:cluster_polls/leased"), metricShapes), nil
}
