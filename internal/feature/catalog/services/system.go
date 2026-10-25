package services

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/catalog/usecases"
	"svc-registry/internal/platform/metrics"
)

func (s *SystemService) FindNode(ctx context.Context, id uuid.UUID) (*catalogtypes.Node, error) {
	return s.nodeRepository.Get(ctx, id)
}

func (s *SystemService) FindChild(ctx context.Context, parent *uuid.UUID, slug string) (*catalogtypes.Node, error) {
	return s.nodeRepository.ChildBySlug(ctx, parent, slug)
}

func (s *SystemService) NodeChain(ctx context.Context, id uuid.UUID) (*catalogtypes.NodeChain, error) {
	return s.nodeRepository.Chain(ctx, catalogtypes.Viewer{}, id)
}

func (s *SystemService) InsertNode(ctx context.Context, n catalogtypes.NewNode) (catalogtypes.Node, error) {
	return s.nodeRepository.Insert(ctx, n.Stored())
}

func (s *SystemService) ChangeNode(ctx context.Context, id uuid.UUID, c catalogtypes.NodeChanges) (catalogtypes.Node, error) {
	return s.nodeRepository.Update(ctx, id, c)
}

func (s *SystemService) PlaceNode(ctx context.Context, id uuid.UUID, parent *uuid.UUID) (catalogtypes.Node, error) {
	return s.moveNode.Run(ctx, id, parent)
}

func (s *SystemService) RemoveNode(ctx context.Context, id uuid.UUID) error {
	return s.nodeRepository.Delete(ctx, id)
}

type SystemService struct {
	nodeRepository     *repositories.NodeRepository
	activityRepository *repositories.ActivityRepository
	moveNode           *usecases.MoveNodeUseCase
	staleDays          uint32
}

var metricShapes = map[string]metrics.Shape{
	"nodes":       {Name: "svcr_catalog_nodes", Labels: []string{"kind"}},
	"branches":    {Name: "svcr_branches", Labels: []string{"state"}},
	"credentials": {Name: "svcr_credentials_active", Labels: []string{"kind"}},
}

func (s *SystemService) MetricSamples(ctx context.Context) ([]metrics.Sample, error) {
	counts, err := s.activityRepository.MetricCounts(ctx, s.staleDays)
	if err != nil {
		return nil, err
	}
	metrics.Zero(counts, "nodes:organization", "nodes:folder", "nodes:project", "branches:active", "branches:stale", "branches:gone",
		"credentials:project_key")
	paths, err := s.activityRepository.ProjectPaths(ctx)
	if err != nil {
		return nil, err
	}
	out := metrics.FromPairs(counts, metricShapes)
	for _, p := range paths {
		out = append(out, metrics.Sample{Name: "svcr_project_info", Value: 1,
			Labels: map[string]string{"project_id": p[0], "organization_id": p[1], "path": p[2]}})
	}
	return out, nil
}
