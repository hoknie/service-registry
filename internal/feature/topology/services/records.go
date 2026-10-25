package services

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/topology/repositories"
	topologytypes "svc-registry/internal/feature/topology/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/metrics"
)

type ClusterRefs struct {
	ProjectID uuid.UUID
	Origin    string
	Refs      []topologytypes.Ref
}

func (s *RecordsService) ReplaceOrigin(ctx context.Context, project uuid.UUID, source topologytypes.Source, origin string, refs []topologytypes.Ref) error {
	targets, err := s.targets(ctx, project, refs)
	if err != nil {
		return apperr.Wrap(err)
	}
	if err := s.dependencyRepository.DeleteOriginExcept(ctx, project, source, origin, topologytypes.TargetKeys(targets)); err != nil {
		return apperr.Wrap(err)
	}
	for _, t := range targets {
		e := topologytypes.NewEdge{ID: uuid.Must(uuid.NewV7()), ProjectID: project, Source: source, Origin: origin, Target: t}
		if err := s.dependencyRepository.Upsert(ctx, e); err != nil {
			return apperr.Wrap(err)
		}
	}
	return nil
}

func (s *RecordsService) ReplaceRepository(ctx context.Context, project uuid.UUID, branch string, refs []topologytypes.Ref) error {
	if err := s.dependencyRepository.DropOtherOrigins(ctx, project, topologytypes.SourceRepository, branch); err != nil {
		return apperr.Wrap(err)
	}
	return s.ReplaceOrigin(ctx, project, topologytypes.SourceRepository, branch, refs)
}

func (s *RecordsService) SyncCluster(ctx context.Context, cluster uuid.UUID, entries []ClusterRefs) error {
	projects := make([]uuid.UUID, 0, len(entries))
	origins := make([]string, 0, len(entries))
	for _, e := range entries {
		projects, origins = append(projects, e.ProjectID), append(origins, e.Origin)
	}
	if err := s.dependencyRepository.KeepClusterOrigins(ctx, cluster, projects, origins); err != nil {
		return apperr.Wrap(err)
	}
	for _, e := range entries {
		if err := s.ReplaceOrigin(ctx, e.ProjectID, topologytypes.SourceCluster, e.Origin, e.Refs); err != nil {
			return err
		}
	}
	return nil
}

func (s *RecordsService) ClearCluster(ctx context.Context, cluster uuid.UUID) error {
	return apperr.Wrap(s.dependencyRepository.KeepClusterOrigins(ctx, cluster, nil, nil))
}

type RecordsService struct {
	resolver
	dependencyRepository *repositories.DependencyRepository
}

func (s *RecordsService) MetricSamples(ctx context.Context) ([]metrics.Sample, error) {
	counts, err := s.dependencyRepository.MetricCounts(ctx)
	if err != nil {
		return nil, err
	}
	return metrics.FromPairs(counts, map[string]metrics.Shape{"covered": {Name: "svcr_projects_covered", Labels: []string{"aspect"}}}), nil
}
