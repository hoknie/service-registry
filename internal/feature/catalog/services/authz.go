package services

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
)

type PathNode struct {
	Node     catalogtypes.Node
	Readable bool
}

type Scoped struct {
	Node       catalogtypes.Node
	Path       []PathNode
	Role       *catalogtypes.Role
	Visibility catalogtypes.Visibility
}

func viewerOf(p accesstypes.Principal) catalogtypes.Viewer {
	return catalogtypes.Viewer{UserID: p.UserID, Nodes: p.Nodes, Admin: p.CatalogAdmin}
}

func newScoped(p accesstypes.Principal, chain catalogtypes.NodeChain) (Scoped, bool) {
	var role *catalogtypes.Role
	if p.IsSuperadmin {
		role = new(catalogtypes.RoleAdmin)
	} else {
		for _, c := range chain.Nodes {
			if c.Bound != nil && (role == nil || *c.Bound > *role) {
				role = new(*c.Bound)
			}
		}
	}
	visibility := catalogtypes.VisibilityNone
	switch {
	case role != nil:
		visibility = catalogtypes.VisibilityRead
	case chain.Navigable:
		visibility = catalogtypes.VisibilityNavigate
	}
	if len(chain.Nodes) == 0 {
		return Scoped{}, false
	}
	readable := p.IsSuperadmin
	path := make([]PathNode, 0, len(chain.Nodes)-1)
	for i := len(chain.Nodes) - 1; i >= 1; i-- {
		c := chain.Nodes[i]
		readable = readable || c.Bound != nil
		path = append(path, PathNode{Node: c.Node, Readable: readable})
	}
	return Scoped{Node: chain.Nodes[0].Node, Path: path, Role: role, Visibility: visibility}, true
}

func (s Scoped) Allows(permission catalogtypes.Permission) bool {
	return s.Role != nil && s.Role.Allows(permission)
}

func (s Scoped) Permissions() []string {
	out := []string{}
	for _, p := range catalogtypes.AllPermissions {
		if s.Allows(p) {
			out = append(out, p.String())
		}
	}
	return out
}

func (s *AccessService) Scope(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (Scoped, error) {
	chain, err := s.nodeRepository.Chain(ctx, viewerOf(p), id)
	if err != nil {
		return Scoped{}, apperr.Wrap(err)
	}
	if chain == nil {
		return Scoped{}, apperr.New(apperr.NotFound)
	}
	sc, ok := newScoped(p, *chain)
	if !ok || sc.Visibility == catalogtypes.VisibilityNone {
		return Scoped{}, apperr.New(apperr.NotFound)
	}
	return sc, nil
}

func (s *AccessService) Authorize(ctx context.Context, p accesstypes.Principal, permission catalogtypes.Permission, id uuid.UUID) (Scoped, error) {
	sc, err := s.Scope(ctx, p, id)
	if err != nil {
		return Scoped{}, err
	}
	if !sc.Allows(permission) {
		return Scoped{}, apperr.New(apperr.Forbidden)
	}
	return sc, nil
}

func (s *AccessService) AuthorizeContainer(ctx context.Context, p accesstypes.Principal, parent, node *uuid.UUID) (*Scoped, error) {
	if parent == nil {
		return nil, accesstypes.RequireCatalogRoot(p, node)
	}
	sc, err := s.Authorize(ctx, p, catalogtypes.PermWrite, *parent)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

func (s *AccessService) CheckRoots(ctx context.Context, owner accesstypes.Principal, ids []uuid.UUID) (bool, error) {
	for _, id := range ids {
		sc, err := s.Scope(ctx, owner, id)
		if errors.Is(err, apperr.ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !sc.Allows(catalogtypes.PermRead) {
			return false, nil
		}
	}
	return true, nil
}

func (s *AccessService) DescribeRoots(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]accesstypes.NodeRef, error) {
	paths, err := s.nodeRepository.Paths(ctx, ids)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make(map[uuid.UUID]accesstypes.NodeRef, len(paths))
	for _, p := range paths {
		out[p.ID] = accesstypes.NodeRef{ID: p.ID, Kind: new(p.Kind), Name: new(p.Name), Path: new(p.Path)}
	}
	return out, nil
}

func (s *AccessService) ReadableProjects(ctx context.Context, p accesstypes.Principal, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]bool{}, nil
	}
	out, err := s.nodeRepository.Readable(ctx, viewerOf(p), p.IsSuperadmin, ids)
	return out, apperr.Wrap(err)
}

type AccessService struct {
	nodeRepository *repositories.NodeRepository
	system         *SystemService
}

func (s *AccessService) ResolveNode(ctx context.Context, p accesstypes.Principal, ref string) (Scoped, error) {
	ref = strings.Trim(strings.TrimSpace(ref), "/")
	id, err := uuid.Parse(ref)
	if err != nil {
		var parent *uuid.UUID
		for _, slug := range strings.Split(ref, "/") {
			n, err := s.system.FindChild(ctx, parent, slug)
			if err != nil {
				return Scoped{}, apperr.Wrap(err)
			}
			if n == nil || slug == "" {
				return Scoped{}, apperr.New(apperr.NotFound)
			}
			parent = &n.ID
		}
		if parent == nil {
			return Scoped{}, apperr.New(apperr.NotFound)
		}
		id = *parent
	}
	scoped, err := s.Authorize(ctx, p, catalogtypes.PermRead, id)
	if err != nil {
		return Scoped{}, apperr.New(apperr.NotFound)
	}
	return scoped, nil
}
