package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/apperr"
)

type PathNode struct {
	Node     catalog.Node
	Readable bool
}

type Scoped struct {
	Node       catalog.Node
	Path       []PathNode
	Role       *catalog.Role
	Visibility catalog.Visibility
}

func newScoped(p access.Principal, chain catalog.NodeChain) (Scoped, bool) {
	var role *catalog.Role
	if p.IsSuperadmin {
		admin := catalog.RoleAdmin
		role = &admin
	} else {
		for _, c := range chain.Nodes {
			if c.Bound != nil && (role == nil || *c.Bound > *role) {
				r := *c.Bound
				role = &r
			}
		}
	}
	visibility := catalog.VisibilityNone
	switch {
	case role != nil:
		visibility = catalog.VisibilityRead
	case chain.Navigable:
		visibility = catalog.VisibilityNavigate
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

func (s Scoped) Allows(permission catalog.Permission) bool {
	return s.Role != nil && s.Role.Allows(permission)
}

func (s Scoped) Permissions() []string {
	out := []string{}
	for _, p := range catalog.AllPermissions {
		if s.Allows(p) {
			out = append(out, p.String())
		}
	}
	return out
}

func (s *Service) Scope(ctx context.Context, p access.Principal, id uuid.UUID) (Scoped, error) {
	chain, err := s.nodes.Chain(ctx, p.UserID, id)
	if err != nil {
		return Scoped{}, apperr.Wrap(err)
	}
	if chain == nil {
		return Scoped{}, apperr.New(apperr.NotFound)
	}
	sc, ok := newScoped(p, *chain)
	if !ok || sc.Visibility == catalog.VisibilityNone {
		return Scoped{}, apperr.New(apperr.NotFound)
	}
	return sc, nil
}

func (s *Service) Authorize(ctx context.Context, p access.Principal, permission catalog.Permission, id uuid.UUID) (Scoped, error) {
	sc, err := s.Scope(ctx, p, id)
	if err != nil {
		return Scoped{}, err
	}
	if !sc.Allows(permission) {
		return Scoped{}, apperr.New(apperr.Forbidden)
	}
	return sc, nil
}

func (s *Service) AuthorizeContainer(ctx context.Context, p access.Principal, parent *uuid.UUID) (*Scoped, error) {
	if parent == nil {
		return nil, access.RequireSuperadmin(p)
	}
	sc, err := s.Authorize(ctx, p, catalog.PermWrite, *parent)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}
