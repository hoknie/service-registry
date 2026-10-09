package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
	"svc-registry/internal/forge"
	"svc-registry/internal/knowledge"
)

const TreeMaxNodes = 1000

func Children(ctx context.Context, state *State, p Principal, parent *uuid.UUID, q access.PageQuery) (access.Page[catalog.WalkNode], error) {
	readable, err := rootReadable(ctx, state, p, parent)
	if err != nil {
		return access.Page[catalog.WalkNode]{}, err
	}
	page, err := access.ValidatePage(q)
	if err != nil {
		return access.Page[catalog.WalkNode]{}, apperr.Wrap(err)
	}
	items, total, err := state.Nodes.Walk(ctx, catalog.Walk{
		UserID: p.UserID, Root: parent, RootReadable: readable, Depth: 1, Limit: page.Limit, Offset: page.Offset,
	})
	if err != nil {
		return access.Page[catalog.WalkNode]{}, apperr.Wrap(err)
	}
	return access.Page[catalog.WalkNode]{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func Tree(ctx context.Context, state *State, p Principal, root *uuid.UUID, depth *int64) ([]catalog.WalkNode, bool, error) {
	readable, err := rootReadable(ctx, state, p, root)
	if err != nil {
		return nil, false, err
	}
	d, err := catalog.ValidateDepth(depth)
	if err != nil {
		return nil, false, apperr.Wrap(err)
	}
	nodes, _, err := state.Nodes.Walk(ctx, catalog.Walk{
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

func rootReadable(ctx context.Context, state *State, p Principal, root *uuid.UUID) (bool, error) {
	if root == nil {
		return p.IsSuperadmin, nil
	}
	s, err := Scope(ctx, state, p, *root)
	if err != nil {
		return false, err
	}
	return s.Allows(catalog.PermRead), nil
}

func GetNode(ctx context.Context, state *State, p Principal, id uuid.UUID) (Scoped, error) {
	return Scope(ctx, state, p, id)
}

func CreateNode(ctx context.Context, state *State, p Principal, in catalog.CreateNode, source *knowledge.SourceInput) (catalog.Node, *IssuedKey, error) {
	parent, err := AuthorizeContainer(ctx, state, p, in.ParentID)
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
	var src *knowledge.NewSource
	if source != nil {
		if kind != catalog.KindProject {
			return catalog.Node{}, nil, apperr.Wrap(catalog.InvalidRepoFieldsNotAllowed)
		}
		n, err := prepareSource(state, row.ID, *source)
		if err != nil {
			return catalog.Node{}, nil, err
		}
		if f, url, ok := sourceRepo(n.Source); ok {
			row.Repo.Forge, row.Repo.RepoURL = f, url
		}
		src = &n
	}
	node, err := state.Nodes.Insert(ctx, row)
	if err != nil {
		return catalog.Node{}, nil, apperr.Wrap(err)
	}
	if kind != catalog.KindProject {
		return node, nil, nil
	}
	if node.Repo.DefaultBranch != nil {
		if err := state.Branches.SetDefault(ctx, node.ID, node.Repo.DefaultBranch); err != nil {
			_ = state.Nodes.Delete(ctx, node.ID)
			return catalog.Node{}, nil, apperr.Wrap(err)
		}
	}
	if src != nil {
		if _, err := state.Sources.Put(ctx, node.ID, *src); err != nil {
			_ = state.Nodes.Delete(ctx, node.ID)
			return catalog.Node{}, nil, apperr.Wrap(err)
		}
	}
	key, err := issueKey(ctx, state, node.ID, 0)
	if err != nil {
		if derr := state.Nodes.Delete(ctx, node.ID); derr != nil {
			return catalog.Node{}, nil, apperr.Wrap(derr)
		}
		return catalog.Node{}, nil, err
	}
	return node, &key, nil
}

func UpdateNode(ctx context.Context, state *State, p Principal, id uuid.UUID, in catalog.UpdateNode) (catalog.Node, error) {
	scoped, err := Authorize(ctx, state, p, catalog.PermWrite, id)
	if err != nil {
		return catalog.Node{}, err
	}
	if in.Description != nil || !in.Repo.IsEmpty() {
		if err := refuseManaged(ctx, state, id); err != nil {
			return catalog.Node{}, err
		}
	}
	var changes catalog.NodeChanges
	if in.Slug != nil {
		s, err := catalog.ValidateSlug(*in.Slug)
		if err != nil {
			return catalog.Node{}, apperr.Wrap(err)
		}
		changes.Slug = &s
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
	node, err := state.Nodes.Update(ctx, id, changes)
	if err != nil {
		return catalog.Node{}, apperr.Wrap(err)
	}
	if changes.DefaultBranch.Set {
		if err := state.Branches.SetDefault(ctx, id, node.Repo.DefaultBranch); err != nil {
			return catalog.Node{}, apperr.Wrap(err)
		}
	}
	return node, nil
}

func MoveNode(ctx context.Context, state *State, p Principal, id uuid.UUID, parent *uuid.UUID) (catalog.Node, error) {
	scoped, err := Scope(ctx, state, p, id)
	if err != nil {
		return catalog.Node{}, err
	}
	if _, err := AuthorizeContainer(ctx, state, p, scoped.Node.ParentID); err != nil {
		return catalog.Node{}, err
	}
	if err := refuseManaged(ctx, state, id); err != nil {
		return catalog.Node{}, err
	}
	if parent != nil && *parent == id {
		return catalog.Node{}, apperr.Wrap(catalog.ConflictCycle)
	}
	target, err := AuthorizeContainer(ctx, state, p, parent)
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
	node, err := state.Nodes.Move(ctx, id, parent)
	return node, apperr.Wrap(err)
}

func DeleteNode(ctx context.Context, state *State, p Principal, id uuid.UUID) error {
	scoped, err := Scope(ctx, state, p, id)
	if err != nil {
		return err
	}
	if _, err := AuthorizeContainer(ctx, state, p, scoped.Node.ParentID); err != nil {
		return err
	}
	if err := refuseManaged(ctx, state, id); err != nil {
		return err
	}
	return apperr.Wrap(state.Nodes.Delete(ctx, id))
}

func refuseManaged(ctx context.Context, state *State, id uuid.UUID) error {
	managed, err := state.Repositories.Managed(ctx, []uuid.UUID{id})
	if err != nil {
		return apperr.Wrap(err)
	}
	if managed[id] {
		return apperr.Wrap(catalog.ConflictManagedByForge)
	}
	return nil
}

func ManagedNodes(ctx context.Context, state *State, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	m, err := state.Repositories.Managed(ctx, ids)
	return m, apperr.Wrap(err)
}

func ProjectRepository(ctx context.Context, state *State, id uuid.UUID) (*forge.Repository, error) {
	r, err := state.Repositories.Find(ctx, id)
	return r, apperr.Wrap(err)
}
