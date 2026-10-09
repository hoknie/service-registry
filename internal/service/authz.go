package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
)

type Principal struct {
	UserID       uuid.UUID
	SessionID    uuid.UUID
	TokenID      uuid.UUID
	Scopes       access.Scopes
	IsSuperadmin bool
	Method       string
}

func (p Principal) ByToken() bool { return p.TokenID != uuid.Nil }

type Need int

const (
	NeedSession Need = iota
	NeedAny
	NeedRead
	NeedWrite
	NeedAdmin
)

func RequireScope(p Principal, need Need) error {
	if !p.ByToken() {
		return nil
	}
	var scope access.Scope
	switch need {
	case NeedAny:
		return nil
	case NeedRead:
		scope = access.ScopeRead
	case NeedWrite:
		scope = access.ScopeWrite
	case NeedAdmin:
		scope = access.ScopeAdmin
	default:
		return apperr.New(apperr.SessionRequired)
	}
	if !p.Scopes.Has(scope) {
		return apperr.New(apperr.InsufficientScope)
	}
	return nil
}

func RequireSuperadmin(p Principal) error {
	if p.IsSuperadmin {
		return nil
	}
	return apperr.New(apperr.Forbidden)
}

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

func newScoped(p Principal, chain catalog.NodeChain) (Scoped, bool) {
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

func Scope(ctx context.Context, state *State, p Principal, id uuid.UUID) (Scoped, error) {
	chain, err := state.Nodes.Chain(ctx, p.UserID, id)
	if err != nil {
		return Scoped{}, apperr.Wrap(err)
	}
	if chain == nil {
		return Scoped{}, apperr.New(apperr.NotFound)
	}
	s, ok := newScoped(p, *chain)
	if !ok || s.Visibility == catalog.VisibilityNone {
		return Scoped{}, apperr.New(apperr.NotFound)
	}
	return s, nil
}

func Authorize(ctx context.Context, state *State, p Principal, permission catalog.Permission, id uuid.UUID) (Scoped, error) {
	s, err := Scope(ctx, state, p, id)
	if err != nil {
		return Scoped{}, err
	}
	if !s.Allows(permission) {
		return Scoped{}, apperr.New(apperr.Forbidden)
	}
	return s, nil
}

func AuthorizeContainer(ctx context.Context, state *State, p Principal, parent *uuid.UUID) (*Scoped, error) {
	if parent == nil {
		return nil, RequireSuperadmin(p)
	}
	s, err := Authorize(ctx, state, p, catalog.PermWrite, *parent)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
