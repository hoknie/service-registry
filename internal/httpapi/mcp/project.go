package mcp

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
	"svc-registry/internal/service"
)

func resolveNode(ctx context.Context, state *service.State, p service.Principal, ref string) (service.Scoped, error) {
	ref = strings.Trim(strings.TrimSpace(ref), "/")
	id, err := uuid.Parse(ref)
	if err != nil {
		var parent *uuid.UUID
		for _, slug := range strings.Split(ref, "/") {
			n, err := state.Nodes.ChildBySlug(ctx, parent, slug)
			if err != nil {
				return service.Scoped{}, apperr.Wrap(err)
			}
			if n == nil || slug == "" {
				return service.Scoped{}, apperr.New(apperr.NotFound)
			}
			parent = &n.ID
		}
		if parent == nil {
			return service.Scoped{}, apperr.New(apperr.NotFound)
		}
		id = *parent
	}
	s, err := service.Authorize(ctx, state, p, catalog.PermRead, id)
	if err != nil {
		return service.Scoped{}, apperr.New(apperr.NotFound)
	}
	return s, nil
}

func resolveProject(ctx context.Context, state *service.State, p service.Principal, ref string) (service.Scoped, error) {
	s, err := resolveNode(ctx, state, p, ref)
	if err != nil {
		return service.Scoped{}, err
	}
	if s.Node.Kind != catalog.KindProject {
		return service.Scoped{}, apperr.New(apperr.NotFound)
	}
	return s, nil
}

func pathOf(s service.Scoped) string {
	parts := make([]string, 0, len(s.Path)+1)
	for _, n := range s.Path {
		parts = append(parts, n.Node.Slug)
	}
	return strings.Join(append(parts, s.Node.Slug), "/")
}
