package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogports "svc-registry/internal/feature/catalog/ports"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
)

func (s *NodesService) CreateNode(ctx context.Context, p accesstypes.Principal, in catalogtypes.CreateNode, source catalogports.SourceFactoryPort) (catalogtypes.Node, *IssuedKey, error) {
	parent, err := s.access.AuthorizeContainer(ctx, p, in.ParentID, nil)
	if err != nil {
		return catalogtypes.Node{}, nil, err
	}
	kind, err := catalogtypes.ValidateKind(in.Kind)
	if err != nil {
		return catalogtypes.Node{}, nil, apperr.Wrap(err)
	}
	var parentKind *catalogtypes.NodeKind
	if parent != nil {
		parentKind = &parent.Node.Kind
	}
	if !kind.MayBeUnder(parentKind) {
		return catalogtypes.Node{}, nil, apperr.Wrap(catalogtypes.ConflictInvalidParent)
	}
	row, err := newNodeRow(in, kind)
	if err != nil {
		return catalogtypes.Node{}, nil, apperr.Wrap(err)
	}
	var src catalogports.ProjectSourcePort
	if source != nil {
		if kind != catalogtypes.KindProject {
			return catalogtypes.Node{}, nil, apperr.Wrap(catalogtypes.InvalidRepoFieldsNotAllowed)
		}
		if src, err = source(row.ID); err != nil {
			return catalogtypes.Node{}, nil, err
		}
		if f, url, ok := src.Repo(); ok {
			row.Repo.Forge, row.Repo.RepoURL = f, url
		}
	}
	node, err := s.insert(ctx, p, row)
	if err != nil {
		return catalogtypes.Node{}, nil, err
	}
	if kind != catalogtypes.KindProject {
		if err := s.mark(ctx, &node); err != nil {
			return catalogtypes.Node{}, nil, err
		}
		return node, nil, nil
	}
	if node.Repo.DefaultBranch != nil {
		if err := s.branchRepository.SetDefault(ctx, node.ID, node.Repo.DefaultBranch); err != nil {
			_ = s.nodeRepository.Delete(ctx, node.ID)
			return catalogtypes.Node{}, nil, apperr.Wrap(err)
		}
	}
	if src != nil {
		if err := src.Attach(ctx, node.ID); err != nil {
			_ = s.nodeRepository.Delete(ctx, node.ID)
			return catalogtypes.Node{}, nil, apperr.Wrap(err)
		}
	}
	key, err := s.projectKeysService.issueKey(ctx, node.ID, 0)
	if err != nil {
		if derr := s.nodeRepository.Delete(ctx, node.ID); derr != nil {
			return catalogtypes.Node{}, nil, apperr.Wrap(derr)
		}
		return catalogtypes.Node{}, nil, err
	}
	if err := s.mark(ctx, &node); err != nil {
		return catalogtypes.Node{}, nil, err
	}
	return node, &key, nil
}

func (s *NodesService) UpdateNode(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in catalogtypes.UpdateNode) (catalogtypes.Node, error) {
	scoped, err := s.access.Authorize(ctx, p, catalogtypes.PermWrite, id)
	if err != nil {
		return catalogtypes.Node{}, err
	}
	if in.Description != nil || !in.Repo.IsEmpty() {
		if err := s.refuseManaged(ctx, id); err != nil {
			return catalogtypes.Node{}, err
		}
	}
	var changes catalogtypes.NodeChanges
	if in.Slug != nil {
		slug, err := catalogtypes.ValidateSlug(*in.Slug)
		if err != nil {
			return catalogtypes.Node{}, apperr.Wrap(err)
		}
		changes.Slug = &slug
	}
	if in.Name != nil {
		n, err := catalogtypes.ValidateName(*in.Name)
		if err != nil {
			return catalogtypes.Node{}, apperr.Wrap(err)
		}
		changes.Name = &n
	}
	if in.Description != nil {
		d, err := catalogtypes.ValidateDescription(*in.Description)
		if err != nil {
			return catalogtypes.Node{}, apperr.Wrap(err)
		}
		changes.Description = &d
	}
	if in.Labels != nil {
		if changes.Labels, err = catalogtypes.ValidateLabels(*in.Labels); err != nil {
			return catalogtypes.Node{}, apperr.Wrap(err)
		}
	}
	if err := catalogtypes.ValidateRepoChanges(scoped.Node.Kind, in.Repo, &changes); err != nil {
		return catalogtypes.Node{}, apperr.Wrap(err)
	}
	if changes.ClusterObservation, err = catalogtypes.ValidateClusterObservation(scoped.Node.Kind, in.ClusterObservation); err != nil {
		return catalogtypes.Node{}, apperr.Wrap(err)
	}
	node, err := s.nodeRepository.Update(ctx, id, changes)
	if err != nil {
		return catalogtypes.Node{}, apperr.Wrap(err)
	}
	if changes.DefaultBranch.Set {
		if err := s.branchRepository.SetDefault(ctx, id, node.Repo.DefaultBranch); err != nil {
			return catalogtypes.Node{}, apperr.Wrap(err)
		}
	}
	if err := s.mark(ctx, &node); err != nil {
		return catalogtypes.Node{}, err
	}
	return node, nil
}

func (s *NodesService) MoveNode(ctx context.Context, p accesstypes.Principal, id uuid.UUID, parent *uuid.UUID) (catalogtypes.Node, error) {
	scoped, err := s.access.Scope(ctx, p, id)
	if err != nil {
		return catalogtypes.Node{}, err
	}
	if _, err := s.access.AuthorizeContainer(ctx, p, scoped.Node.ParentID, &id); err != nil {
		return catalogtypes.Node{}, err
	}
	if err := s.refuseManaged(ctx, id); err != nil {
		return catalogtypes.Node{}, err
	}
	if parent != nil && *parent == id {
		return catalogtypes.Node{}, apperr.Wrap(catalogtypes.ConflictCycle)
	}
	target, err := s.access.AuthorizeContainer(ctx, p, parent, &id)
	if err != nil {
		return catalogtypes.Node{}, err
	}
	var targetKind *catalogtypes.NodeKind
	if target != nil {
		targetKind = &target.Node.Kind
	}
	if !scoped.Node.Kind.MayBeUnder(targetKind) {
		return catalogtypes.Node{}, apperr.Wrap(catalogtypes.ConflictInvalidParent)
	}
	node, err := s.nodeRepository.Move(ctx, id, parent)
	if err != nil {
		return catalogtypes.Node{}, apperr.Wrap(err)
	}
	if err := s.mark(ctx, &node); err != nil {
		return catalogtypes.Node{}, err
	}
	return node, nil
}

func (s *NodesService) DeleteNode(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	scoped, err := s.access.Scope(ctx, p, id)
	if err != nil {
		return err
	}
	if _, err := s.access.AuthorizeContainer(ctx, p, scoped.Node.ParentID, &id); err != nil {
		return err
	}
	if err := s.refuseManaged(ctx, id); err != nil {
		return err
	}
	return apperr.Wrap(s.nodeRepository.Delete(ctx, id))
}

func (s *NodesService) insert(ctx context.Context, p accesstypes.Principal, row catalogtypes.NewNode) (catalogtypes.Node, error) {
	if row.ParentID != nil || !p.AddsRoots() {
		node, err := s.nodeRepository.Insert(ctx, row)
		return node, apperr.Wrap(err)
	}
	if len(p.Nodes) >= accesstypes.TokenMaxNodes {
		return catalogtypes.Node{}, apperr.Wrap(catalogtypes.ConflictTokenNodesFull)
	}
	var node catalogtypes.Node
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		var err error
		if node, err = s.nodeRepository.Insert(ctx, row); err != nil {
			return err
		}
		added, err := s.tokenRoots.AddTokenRoot(ctx, p.TokenID, node.ID)
		if err != nil {
			return err
		}
		if !added {
			return catalogtypes.ConflictTokenNodesFull
		}
		return nil
	})
	return node, apperr.Wrap(err)
}

func (s *NodesService) refuseManaged(ctx context.Context, id uuid.UUID) error {
	managed, err := s.managed.Managed(ctx, []uuid.UUID{id})
	if err != nil {
		return apperr.Wrap(err)
	}
	if managed[id] {
		return apperr.Wrap(catalogtypes.ConflictManagedByForge)
	}
	return nil
}

func newNodeRow(in catalogtypes.CreateNode, kind catalogtypes.NodeKind) (catalogtypes.NewNode, error) {
	row := catalogtypes.NewNode{ID: uuid.Must(uuid.NewV7()), Kind: kind, ParentID: in.ParentID, Labels: catalogtypes.Labels{}}
	var err error
	if row.Slug, err = catalogtypes.ValidateSlug(in.Slug); err != nil {
		return row, err
	}
	if row.Name, err = catalogtypes.ValidateName(in.Name); err != nil {
		return row, err
	}
	description := ""
	if in.Description != nil {
		description = *in.Description
	}
	if row.Description, err = catalogtypes.ValidateDescription(description); err != nil {
		return row, err
	}
	if in.Labels != nil {
		if row.Labels, err = catalogtypes.ValidateLabels(*in.Labels); err != nil {
			return row, err
		}
	}
	if row.Repo, err = catalogtypes.ValidateRepo(kind, in.Repo); err != nil {
		return row, err
	}
	row.ClusterObservation, err = catalogtypes.ValidateClusterObservation(kind, in.ClusterObservation)
	return row, err
}
