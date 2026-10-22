package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogports "svc-registry/internal/feature/catalog/ports"
	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
)

const TreeMaxNodes = 1000

func (s *NodesService) Children(ctx context.Context, p accesstypes.Principal, parent *uuid.UUID, q accesstypes.PageQuery) (accesstypes.Page[catalogtypes.WalkNode], error) {
	readable, err := s.rootReadable(ctx, p, parent)
	if err != nil {
		return accesstypes.Page[catalogtypes.WalkNode]{}, err
	}
	page, err := accesstypes.ValidatePage(q)
	if err != nil {
		return accesstypes.Page[catalogtypes.WalkNode]{}, apperr.Wrap(err)
	}
	items, total, err := s.nodeRepository.Walk(ctx, catalogtypes.Walk{
		UserID: p.UserID, Root: parent, RootReadable: readable, Depth: 1, Limit: page.Limit, Offset: page.Offset,
	})
	if err != nil {
		return accesstypes.Page[catalogtypes.WalkNode]{}, apperr.Wrap(err)
	}
	if err := s.markWalked(ctx, items); err != nil {
		return accesstypes.Page[catalogtypes.WalkNode]{}, err
	}
	return accesstypes.Page[catalogtypes.WalkNode]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *NodesService) Tree(ctx context.Context, p accesstypes.Principal, root *uuid.UUID, depth *int64) ([]catalogtypes.WalkNode, bool, error) {
	readable, err := s.rootReadable(ctx, p, root)
	if err != nil {
		return nil, false, err
	}
	d, err := catalogtypes.ValidateDepth(depth)
	if err != nil {
		return nil, false, apperr.Wrap(err)
	}
	nodes, _, err := s.nodeRepository.Walk(ctx, catalogtypes.Walk{
		UserID: p.UserID, Root: root, RootReadable: readable, Depth: d, Limit: TreeMaxNodes + 1,
	})
	if err != nil {
		return nil, false, apperr.Wrap(err)
	}
	truncated := len(nodes) > TreeMaxNodes
	if truncated {
		nodes = nodes[:TreeMaxNodes]
	}
	if err := s.markWalked(ctx, nodes); err != nil {
		return nil, false, err
	}
	return nodes, truncated, nil
}

func (s *NodesService) rootReadable(ctx context.Context, p accesstypes.Principal, root *uuid.UUID) (bool, error) {
	if root == nil {
		return p.IsSuperadmin, nil
	}
	sc, err := s.access.Scope(ctx, p, *root)
	if err != nil {
		return false, err
	}
	return sc.Allows(catalogtypes.PermRead), nil
}

func (s *NodesService) GetNode(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (Scoped, error) {
	scoped, err := s.access.Scope(ctx, p, id)
	if err != nil || scoped.Role == nil {
		return scoped, err
	}
	if err := s.mark(ctx, &scoped.Node); err != nil {
		return Scoped{}, err
	}
	return scoped, nil
}

func (s *NodesService) mark(ctx context.Context, node *catalogtypes.Node) error {
	managed, err := s.managed.Managed(ctx, []uuid.UUID{node.ID})
	if err != nil {
		return apperr.Wrap(err)
	}
	node.Managed = managed[node.ID]
	return nil
}

func (s *NodesService) markWalked(ctx context.Context, nodes []catalogtypes.WalkNode) error {
	ids := make([]uuid.UUID, 0, len(nodes))
	for _, n := range nodes {
		if n.Readable {
			ids = append(ids, n.Node.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	managed, err := s.managed.Managed(ctx, ids)
	if err != nil {
		return apperr.Wrap(err)
	}
	for i := range nodes {
		nodes[i].Node.Managed = managed[nodes[i].Node.ID]
	}
	return nil
}

type NodesService struct {
	branchRepository   *repositories.BranchRepository
	managed            catalogports.ManagedNodesPort
	nodeRepository     *repositories.NodeRepository
	access             *AccessService
	projectKeysService *ProjectKeysService
}
