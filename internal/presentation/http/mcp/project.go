package mcp

import (
	"context"
	"strings"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
)

func resolveProject(ctx context.Context, deps Deps, p accesstypes.Principal, ref string) (catalogservices.Scoped, error) {
	s, err := deps.Catalog.Access.ResolveNode(ctx, p, ref)
	if err != nil {
		return catalogservices.Scoped{}, err
	}
	if s.Node.Kind != catalogtypes.KindProject {
		return catalogservices.Scoped{}, apperr.New(apperr.NotFound)
	}
	return s, nil
}

func pathOf(s catalogservices.Scoped) string {
	parts := make([]string, 0, len(s.Path)+1)
	for _, n := range s.Path {
		parts = append(parts, n.Node.Slug)
	}
	return strings.Join(append(parts, s.Node.Slug), "/")
}
