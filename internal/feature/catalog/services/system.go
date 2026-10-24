package services

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/catalog/usecases"
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
	nodeRepository *repositories.NodeRepository
	moveNode       *usecases.MoveNodeUseCase
}
