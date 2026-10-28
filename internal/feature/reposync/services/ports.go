package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/reposync/repositories"
	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/internal/platform/metrics"
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
		v, err := s.secrets.Open(row.WebhookSecretEnc, secretbox.AAD(reposynctypes.LegacyTable, row.ID.String(), reposynctypes.ColumnWebhookSecret))
		if err != nil {
			return nil, 0, err
		}
		plain[i] = v
	}
	apply := func(ctx context.Context) error {
		for i, row := range rows {
			enc, err := s.secrets.Seal(plain[i], secretbox.AAD(reposynctypes.LegacyTable, row.ID.String(), reposynctypes.ColumnWebhookSecret))
			if err != nil {
				return err
			}
			if err := s.connectionRepository.ReplaceWebhookSecret(ctx, reposynctypes.SecretRow{ID: row.ID, WebhookSecretEnc: enc}); err != nil {
				return err
			}
		}
		return nil
	}
	return apply, len(rows), nil
}

func (s *PortsService) AdoptLegacyCredentials(ctx context.Context, adopt catalogtypes.AdoptFunc) (int, error) {
	rows, err := s.legacy(ctx)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		value := catalogtypes.SecretValue{Ref: row.Ref}
		if row.Ref == nil {
			v, err := s.secrets.Open(*row.Enc, secretbox.AAD(reposynctypes.LegacyTable, row.ID.String(), reposynctypes.ColumnCredentials))
			if err != nil {
				return 0, fmt.Errorf("repository sync connection %s: %w", row.ID, err)
			}
			value.Value = &v
		}
		secret, err := adopt(ctx, catalogtypes.LegacyCredential{RecordKind: "repo", RecordID: row.ID, Scope: &row.NodeID,
			Name: row.OwnerPath, Value: value})
		if err != nil {
			return 0, fmt.Errorf("repository sync connection %s: %w", row.ID, err)
		}
		if err := s.connectionRepository.AdoptSecret(ctx, row.ID, secret); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

func (s *PortsService) LegacyCredentialCount(ctx context.Context) (int, error) {
	rows, err := s.legacy(ctx)
	return len(rows), err
}

func (s *PortsService) legacy(ctx context.Context) ([]reposynctypes.LegacyConnection, error) {
	ok, err := s.connectionRepository.HasLegacyColumns(ctx)
	if err != nil || !ok {
		return nil, err
	}
	return s.connectionRepository.Legacy(ctx)
}

type PortsService struct {
	connectionRepository *repositories.ConnectionRepository
	repoRepository       *repositories.RepoRepository
	secrets              *secretbox.Box
}

var metricShapes = map[string]metrics.Shape{
	"connections": {Name: "svcr_repo_sync_connections", Labels: []string{"hosting", "state"}},
	"covered":     {Name: "svcr_projects_covered", Labels: []string{"aspect"}},
	"queue":       {Name: "svcr_job_queue", Labels: []string{"job", "state"}},
}

func (s *PortsService) MetricSamples(ctx context.Context) ([]metrics.Sample, error) {
	counts, err := s.connectionRepository.MetricCounts(ctx)
	if err != nil {
		return nil, err
	}
	return metrics.FromPairs(counts, metricShapes), nil
}

func (s *PortsService) QueueSamples(ctx context.Context) ([]metrics.Sample, error) {
	counts, err := s.connectionRepository.QueueCounts(ctx)
	if err != nil {
		return nil, err
	}
	return metrics.FromPairs(metrics.Zero(counts, "queue:repo_sync/due", "queue:repo_sync/leased"), metricShapes), nil
}
