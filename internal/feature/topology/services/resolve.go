package services

import (
	"context"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	topologytypes "svc-registry/internal/feature/topology/types"
)

type resolver struct {
	catalog *catalogservices.CatalogServices
}

func (r resolver) project(ctx context.Context, ref topologytypes.Ref) (*catalogtypes.Node, error) {
	if ref.ID != nil {
		n, err := r.catalog.System.FindNode(ctx, *ref.ID)
		if err != nil || n == nil || n.Kind != catalogtypes.KindProject {
			return nil, err
		}
		return n, nil
	}
	var n *catalogtypes.Node
	var parent *uuid.UUID
	for _, slug := range ref.Path {
		var err error
		if n, err = r.catalog.System.FindChild(ctx, parent, slug); err != nil || n == nil {
			return nil, err
		}
		parent = &n.ID
	}
	if n == nil || n.Kind != catalogtypes.KindProject {
		return nil, nil
	}
	return n, nil
}

func (r resolver) targets(ctx context.Context, self uuid.UUID, refs []topologytypes.Ref) ([]topologytypes.NewTarget, error) {
	out := make([]topologytypes.NewTarget, 0, len(refs))
	var found []uuid.UUID
	for _, ref := range refs {
		if ref.External != "" {
			out = append(out, topologytypes.NewTarget{External: new(ref.External), Ref: ref.Text()})
			continue
		}
		n, err := r.project(ctx, ref)
		if err != nil {
			return nil, err
		}
		switch {
		case n == nil:
			out = append(out, topologytypes.NewTarget{Ref: ref.Text()})
		case n.ID != self:
			out = append(out, topologytypes.NewTarget{ProjectID: new(n.ID), Ref: ref.Text()})
			found = append(found, n.ID)
		}
	}
	paths, err := r.catalog.Access.DescribeRoots(ctx, found)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	kept := out[:0]
	for _, t := range out {
		if t.ProjectID != nil {
			if d, ok := paths[*t.ProjectID]; ok && d.Path != nil {
				t.Ref = *d.Path
			}
		}
		key := t.Ref
		if t.ProjectID != nil {
			key = t.ProjectID.String()
		} else if t.External != nil {
			key = "external:" + *t.External
		}
		if !seen[key] {
			seen[key] = true
			kept = append(kept, t)
		}
	}
	return kept, nil
}
