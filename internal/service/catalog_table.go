package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
	"svc-registry/internal/links"
)

type CatalogTablePage struct {
	Items     []catalog.TableNode
	Total     uint64
	Limit     uint32
	Offset    uint64
	Truncated bool
	Processes map[uuid.UUID][]catalog.Process
	Summaries map[uuid.UUID]catalog.Summary
	Links     map[uuid.UUID][]links.Badge
}

func CatalogTable(ctx context.Context, state *State, p Principal, parent *uuid.UUID, pq access.PageQuery, tq catalog.TableQuery) (CatalogTablePage, error) {
	readable, err := rootReadable(ctx, state, p, parent)
	if err != nil {
		return CatalogTablePage{}, err
	}
	filter, err := catalog.ValidateTable(tq)
	if err != nil {
		return CatalogTablePage{}, apperr.Wrap(err)
	}
	var out CatalogTablePage
	if filter.Active() {
		out, err = searchTable(ctx, state, p, parent, readable, filter)
	} else {
		out, err = childrenTable(ctx, state, p, parent, readable, pq)
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
	out.Processes, out.Summaries, err = activities(ctx, state, p, projects, containers)
	if err != nil {
		return CatalogTablePage{}, err
	}
	if out.Links, err = projectBadges(ctx, state, projects); err != nil {
		return CatalogTablePage{}, err
	}
	return out, nil
}

func projectBadges(ctx context.Context, state *State, projects []uuid.UUID) (map[uuid.UUID][]links.Badge, error) {
	data, err := state.LinkTemplates.ProjectsData(ctx, projects)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make(map[uuid.UUID][]links.Badge, len(projects))
	for _, id := range projects {
		b := data[id]
		out[id] = links.Badges(b.Context(id), b.Templates)
	}
	return out, nil
}

func childrenTable(ctx context.Context, state *State, p Principal, parent *uuid.UUID, readable bool, pq access.PageQuery) (CatalogTablePage, error) {
	page, err := access.ValidatePage(pq)
	if err != nil {
		return CatalogTablePage{}, apperr.Wrap(err)
	}
	items, total, err := state.Nodes.Table(ctx, catalog.Walk{
		UserID: p.UserID, Root: parent, RootReadable: readable, Depth: 1, Limit: page.Limit, Offset: page.Offset,
	})
	if err != nil {
		return CatalogTablePage{}, apperr.Wrap(err)
	}
	return CatalogTablePage{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func searchTable(ctx context.Context, state *State, p Principal, parent *uuid.UUID, readable bool, filter catalog.TableFilter) (CatalogTablePage, error) {
	var only []uuid.UUID
	if filter.Activity != "" {
		busy, err := state.Activity.Busy(ctx, activityWant(state))
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
	items, matches, err := state.Nodes.Search(ctx, catalog.Search{
		UserID: p.UserID, Root: parent, RootReadable: readable, Filter: filter, Only: only, Limit: catalog.TableMaxMatches,
	})
	if err != nil {
		return CatalogTablePage{}, apperr.Wrap(err)
	}
	return CatalogTablePage{Items: items, Total: uint64(len(items)), Limit: uint32(len(items)),
		Truncated: matches > catalog.TableMaxMatches}, nil
}
