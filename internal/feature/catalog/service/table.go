package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/apperr"
)

type CatalogTablePage struct {
	Items     []catalog.TableNode
	Total     uint64
	Limit     uint32
	Offset    uint64
	Truncated bool
	Processes map[uuid.UUID][]catalog.Process
	Summaries map[uuid.UUID]catalog.Summary
}

func (s *Service) CatalogTable(ctx context.Context, p access.Principal, parent *uuid.UUID, pq access.PageQuery, tq catalog.TableQuery) (CatalogTablePage, error) {
	readable, err := s.rootReadable(ctx, p, parent)
	if err != nil {
		return CatalogTablePage{}, err
	}
	filter, err := catalog.ValidateTable(tq)
	if err != nil {
		return CatalogTablePage{}, apperr.Wrap(err)
	}
	var out CatalogTablePage
	if filter.Active() {
		out, err = s.searchTable(ctx, p, parent, readable, filter)
	} else {
		out, err = s.childrenTable(ctx, p, parent, readable, pq)
	}
	if err != nil {
		return CatalogTablePage{}, err
	}
	var projects, containers []uuid.UUID
	for _, n := range out.Items {
		switch {
		case !n.Readable:
		case n.Node.Kind == catalog.KindProject:
			projects = append(projects, n.Node.ID)
		default:
			containers = append(containers, n.Node.ID)
		}
	}
	out.Processes, out.Summaries, err = s.activities(ctx, p, projects, containers)
	if err != nil {
		return CatalogTablePage{}, err
	}
	return out, nil
}

func (s *Service) childrenTable(ctx context.Context, p access.Principal, parent *uuid.UUID, readable bool, pq access.PageQuery) (CatalogTablePage, error) {
	page, err := access.ValidatePage(pq)
	if err != nil {
		return CatalogTablePage{}, apperr.Wrap(err)
	}
	items, total, err := s.nodes.Table(ctx, catalog.Walk{
		UserID: p.UserID, Root: parent, RootReadable: readable, Depth: 1, Limit: page.Limit, Offset: page.Offset,
	})
	if err != nil {
		return CatalogTablePage{}, apperr.Wrap(err)
	}
	return CatalogTablePage{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *Service) searchTable(ctx context.Context, p access.Principal, parent *uuid.UUID, readable bool, filter catalog.TableFilter) (CatalogTablePage, error) {
	var only []uuid.UUID
	if filter.Activity != "" {
		busy, err := s.signals(ctx, nil, false, true)
		if err != nil {
			return CatalogTablePage{}, apperr.Wrap(err)
		}
		only = []uuid.UUID{}
		for id, signals := range busy {
			if catalog.Has(catalog.ProcessesOf(signals), filter.Activity) {
				only = append(only, id)
			}
		}
	}
	items, matches, err := s.nodes.Search(ctx, catalog.Search{
		UserID: p.UserID, Root: parent, RootReadable: readable, Filter: filter, Only: only, Limit: catalog.TableMaxMatches,
	})
	if err != nil {
		return CatalogTablePage{}, apperr.Wrap(err)
	}
	return CatalogTablePage{Items: items, Total: uint64(len(items)), Limit: uint32(len(items)),
		Truncated: matches > catalog.TableMaxMatches}, nil
}

func (p CatalogTablePage) ReadableProjects() []uuid.UUID {
	var out []uuid.UUID
	for _, n := range p.Items {
		if n.Readable && n.Node.Kind == catalog.KindProject {
			out = append(out, n.Node.ID)
		}
	}
	return out
}
