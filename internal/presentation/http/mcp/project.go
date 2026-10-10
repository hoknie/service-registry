package mcp

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	catalogservice "svc-registry/internal/feature/catalog/service"
	"svc-registry/internal/platform/apperr"
)

func resolveNode(ctx context.Context, deps Deps, p access.Principal, ref string) (catalogservice.Scoped, error) {
	ref = strings.Trim(strings.TrimSpace(ref), "/")
	id, err := uuid.Parse(ref)
	if err != nil {
		var parent *uuid.UUID
		for _, slug := range strings.Split(ref, "/") {
			n, err := deps.Catalog.FindChild(ctx, parent, slug)
			if err != nil {
				return catalogservice.Scoped{}, apperr.Wrap(err)
			}
			if n == nil || slug == "" {
				return catalogservice.Scoped{}, apperr.New(apperr.NotFound)
			}
			parent = &n.ID
		}
		if parent == nil {
			return catalogservice.Scoped{}, apperr.New(apperr.NotFound)
		}
		id = *parent
	}
	s, err := deps.Catalog.Authorize(ctx, p, catalog.PermRead, id)
	if err != nil {
		return catalogservice.Scoped{}, apperr.New(apperr.NotFound)
	}
	return s, nil
}

func resolveProject(ctx context.Context, deps Deps, p access.Principal, ref string) (catalogservice.Scoped, error) {
	s, err := resolveNode(ctx, deps, p, ref)
	if err != nil {
		return catalogservice.Scoped{}, err
	}
	if s.Node.Kind != catalog.KindProject {
		return catalogservice.Scoped{}, apperr.New(apperr.NotFound)
	}
	return s, nil
}

func pathOf(s catalogservice.Scoped) string {
	parts := make([]string, 0, len(s.Path)+1)
	for _, n := range s.Path {
		parts = append(parts, n.Node.Slug)
	}
	return strings.Join(append(parts, s.Node.Slug), "/")
}
