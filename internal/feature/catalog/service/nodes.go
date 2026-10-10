package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/apperr"
)

const TreeMaxNodes = 1000

func (s *Service) Children(ctx context.Context, p access.Principal, parent *uuid.UUID, q access.PageQuery) (access.Page[catalog.WalkNode], error) {
	readable, err := s.rootReadable(ctx, p, parent)
	if err != nil {
		return access.Page[catalog.WalkNode]{}, err
	}
	page, err := access.ValidatePage(q)
	if err != nil {
		return access.Page[catalog.WalkNode]{}, apperr.Wrap(err)
	}
	items, total, err := s.nodes.Walk(ctx, catalog.Walk{
		UserID: p.UserID, Root: parent, RootReadable: readable, Depth: 1, Limit: page.Limit, Offset: page.Offset,
	})
	if err != nil {
		return access.Page[catalog.WalkNode]{}, apperr.Wrap(err)
	}
	return access.Page[catalog.WalkNode]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *Service) Tree(ctx context.Context, p access.Principal, root *uuid.UUID, depth *int64) ([]catalog.WalkNode, bool, error) {
	readable, err := s.rootReadable(ctx, p, root)
	if err != nil {
		return nil, false, err
	}
	d, err := catalog.ValidateDepth(depth)
	if err != nil {
		return nil, false, apperr.Wrap(err)
	}
	nodes, _, err := s.nodes.Walk(ctx, catalog.Walk{
		UserID: p.UserID, Root: root, RootReadable: readable, Depth: d, Limit: TreeMaxNodes + 1,
	})
	if err != nil {
		return nil, false, apperr.Wrap(err)
	}
	truncated := len(nodes) > TreeMaxNodes
	if truncated {
		nodes = nodes[:TreeMaxNodes]
	}
	return nodes, truncated, nil
}

func (s *Service) rootReadable(ctx context.Context, p access.Principal, root *uuid.UUID) (bool, error) {
	if root == nil {
		return p.IsSuperadmin, nil
	}
	sc, err := s.Scope(ctx, p, *root)
	if err != nil {
		return false, err
	}
	return sc.Allows(catalog.PermRead), nil
}

func (s *Service) GetNode(ctx context.Context, p access.Principal, id uuid.UUID) (Scoped, error) {
	return s.Scope(ctx, p, id)
}

func (s *Service) CreateNode(ctx context.Context, p access.Principal, in catalog.CreateNode, source catalog.SourceFactory) (catalog.Node, *IssuedKey, error) {
	parent, err := s.AuthorizeContainer(ctx, p, in.ParentID)
	if err != nil {
		return catalog.Node{}, nil, err
	}
	kind, err := catalog.ValidateKind(in.Kind)
	if err != nil {
		return catalog.Node{}, nil, apperr.Wrap(err)
	}
	var parentKind *catalog.NodeKind
	if parent != nil {
		parentKind = &parent.Node.Kind
	}
	if !kind.MayBeUnder(parentKind) {
		return catalog.Node{}, nil, apperr.Wrap(catalog.ConflictInvalidParent)
	}
	row := catalog.NewNode{ID: uuid.Must(uuid.NewV7()), Kind: kind, ParentID: in.ParentID, Labels: catalog.Labels{}}
	if row.Slug, err = catalog.ValidateSlug(in.Slug); err != nil {
		return catalog.Node{}, nil, apperr.Wrap(err)
	}
	if row.Name, err = catalog.ValidateName(in.Name); err != nil {
		return catalog.Node{}, nil, apperr.Wrap(err)
	}
	description := ""
	if in.Description != nil {
		description = *in.Description
	}
	if row.Description, err = catalog.ValidateDescription(description); err != nil {
		return catalog.Node{}, nil, apperr.Wrap(err)
	}
	if in.Labels != nil {
		if row.Labels, err = catalog.ValidateLabels(*in.Labels); err != nil {
			return catalog.Node{}, nil, apperr.Wrap(err)
		}
	}
	if row.Repo, err = catalog.ValidateRepo(kind, in.Repo); err != nil {
		return catalog.Node{}, nil, apperr.Wrap(err)
	}
	if row.ClusterObservation, err = catalog.ValidateClusterObservation(kind, in.ClusterObservation); err != nil {
		return catalog.Node{}, nil, apperr.Wrap(err)
	}
	var src catalog.ProjectSource
	if source != nil {
		if kind != catalog.KindProject {
			return catalog.Node{}, nil, apperr.Wrap(catalog.InvalidRepoFieldsNotAllowed)
		}
		if src, err = source(row.ID); err != nil {
			return catalog.Node{}, nil, err
		}
		if f, url, ok := src.Repo(); ok {
			row.Repo.Forge, row.Repo.RepoURL = f, url
		}
	}
	node, err := s.nodes.Insert(ctx, row)
	if err != nil {
		return catalog.Node{}, nil, apperr.Wrap(err)
	}
	if kind != catalog.KindProject {
		return node, nil, nil
	}
	if node.Repo.DefaultBranch != nil {
		if err := s.branches.SetDefault(ctx, node.ID, node.Repo.DefaultBranch); err != nil {
			_ = s.nodes.Delete(ctx, node.ID)
			return catalog.Node{}, nil, apperr.Wrap(err)
		}
	}
	if src != nil {
		if err := src.Attach(ctx, node.ID); err != nil {
			_ = s.nodes.Delete(ctx, node.ID)
			return catalog.Node{}, nil, apperr.Wrap(err)
		}
	}
	key, err := s.issueKey(ctx, node.ID, 0)
	if err != nil {
		if derr := s.nodes.Delete(ctx, node.ID); derr != nil {
			return catalog.Node{}, nil, apperr.Wrap(derr)
		}
		return catalog.Node{}, nil, err
	}
	return node, &key, nil
}

func (s *Service) UpdateNode(ctx context.Context, p access.Principal, id uuid.UUID, in catalog.UpdateNode) (catalog.Node, error) {
	scoped, err := s.Authorize(ctx, p, catalog.PermWrite, id)
	if err != nil {
		return catalog.Node{}, err
	}
	if in.Description != nil || !in.Repo.IsEmpty() {
		if err := s.refuseManaged(ctx, id); err != nil {
			return catalog.Node{}, err
		}
	}
	var changes catalog.NodeChanges
	if in.Slug != nil {
		slug, err := catalog.ValidateSlug(*in.Slug)
		if err != nil {
			return catalog.Node{}, apperr.Wrap(err)
		}
		changes.Slug = &slug
	}
	if in.Name != nil {
		n, err := catalog.ValidateName(*in.Name)
		if err != nil {
			return catalog.Node{}, apperr.Wrap(err)
		}
		changes.Name = &n
	}
	if in.Description != nil {
		d, err := catalog.ValidateDescription(*in.Description)
		if err != nil {
			return catalog.Node{}, apperr.Wrap(err)
		}
		changes.Description = &d
	}
	if in.Labels != nil {
		if changes.Labels, err = catalog.ValidateLabels(*in.Labels); err != nil {
			return catalog.Node{}, apperr.Wrap(err)
		}
	}
	if err := catalog.ValidateRepoChanges(scoped.Node.Kind, in.Repo, &changes); err != nil {
		return catalog.Node{}, apperr.Wrap(err)
	}
	if changes.ClusterObservation, err = catalog.ValidateClusterObservation(scoped.Node.Kind, in.ClusterObservation); err != nil {
		return catalog.Node{}, apperr.Wrap(err)
	}
	node, err := s.nodes.Update(ctx, id, changes)
	if err != nil {
		return catalog.Node{}, apperr.Wrap(err)
	}
	if changes.DefaultBranch.Set {
		if err := s.branches.SetDefault(ctx, id, node.Repo.DefaultBranch); err != nil {
			return catalog.Node{}, apperr.Wrap(err)
		}
	}
	return node, nil
}

func (s *Service) MoveNode(ctx context.Context, p access.Principal, id uuid.UUID, parent *uuid.UUID) (catalog.Node, error) {
	scoped, err := s.Scope(ctx, p, id)
	if err != nil {
		return catalog.Node{}, err
	}
	if _, err := s.AuthorizeContainer(ctx, p, scoped.Node.ParentID); err != nil {
		return catalog.Node{}, err
	}
	if err := s.refuseManaged(ctx, id); err != nil {
		return catalog.Node{}, err
	}
	if parent != nil && *parent == id {
		return catalog.Node{}, apperr.Wrap(catalog.ConflictCycle)
	}
	target, err := s.AuthorizeContainer(ctx, p, parent)
	if err != nil {
		return catalog.Node{}, err
	}
	var targetKind *catalog.NodeKind
	if target != nil {
		targetKind = &target.Node.Kind
	}
	if !scoped.Node.Kind.MayBeUnder(targetKind) {
		return catalog.Node{}, apperr.Wrap(catalog.ConflictInvalidParent)
	}
	node, err := s.nodes.Move(ctx, id, parent)
	return node, apperr.Wrap(err)
}

func (s *Service) DeleteNode(ctx context.Context, p access.Principal, id uuid.UUID) error {
	scoped, err := s.Scope(ctx, p, id)
	if err != nil {
		return err
	}
	if _, err := s.AuthorizeContainer(ctx, p, scoped.Node.ParentID); err != nil {
		return err
	}
	if err := s.refuseManaged(ctx, id); err != nil {
		return err
	}
	return apperr.Wrap(s.nodes.Delete(ctx, id))
}

func (s *Service) refuseManaged(ctx context.Context, id uuid.UUID) error {
	managed, err := s.managed.Managed(ctx, []uuid.UUID{id})
	if err != nil {
		return apperr.Wrap(err)
	}
	if managed[id] {
		return apperr.Wrap(catalog.ConflictManagedByForge)
	}
	return nil
}
