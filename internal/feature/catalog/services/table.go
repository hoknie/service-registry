package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogports "svc-registry/internal/feature/catalog/ports"
	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
)

type CatalogTablePage struct {
	Items     []catalogtypes.TableNode
	Total     uint64
	Limit     uint32
	Offset    uint64
	Truncated bool
	Processes map[uuid.UUID][]catalogtypes.Process
	Summaries map[uuid.UUID]catalogtypes.Summary
}

func (s *TableService) CatalogTable(ctx context.Context, p accesstypes.Principal, parent *uuid.UUID, pq accesstypes.PageQuery, tq catalogtypes.TableQuery) (CatalogTablePage, error) {
	readable, err := s.nodesService.rootReadable(ctx, p, parent)
	if err != nil {
		return CatalogTablePage{}, err
	}
	filter, err := catalogtypes.ValidateTable(tq)
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
		case n.Node.Kind == catalogtypes.KindProject:
			projects = append(projects, n.Node.ID)
		default:
			containers = append(containers, n.Node.ID)
		}
	}
	out.Processes, out.Summaries, err = s.activityService.activities(ctx, p, projects, containers)
	if err != nil {
		return CatalogTablePage{}, err
	}
	walked := make([]catalogtypes.WalkNode, len(out.Items))
	for i, n := range out.Items {
		walked[i] = n.WalkNode
	}
	if err := s.nodesService.markWalked(ctx, walked); err != nil {
		return CatalogTablePage{}, err
	}
	for i := range out.Items {
		out.Items[i].Node.Managed = walked[i].Node.Managed
	}
	return out, nil
}

func (s *TableService) childrenTable(ctx context.Context, p accesstypes.Principal, parent *uuid.UUID, readable bool, pq accesstypes.PageQuery) (CatalogTablePage, error) {
	page, err := accesstypes.ValidatePage(pq)
	if err != nil {
		return CatalogTablePage{}, apperr.Wrap(err)
	}
	items, total, err := s.nodeRepository.Table(ctx, catalogtypes.Walk{
		Viewer: viewerOf(p), Root: parent, RootReadable: readable, Depth: 1, Limit: page.Limit, Offset: page.Offset,
	})
	if err != nil {
		return CatalogTablePage{}, apperr.Wrap(err)
	}
	return CatalogTablePage{Items: items, Total: total, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *TableService) searchTable(ctx context.Context, p accesstypes.Principal, parent *uuid.UUID, readable bool, filter catalogtypes.TableFilter) (CatalogTablePage, error) {
	only, err := s.onlyProjects(ctx, p, filter)
	if err != nil {
		return CatalogTablePage{}, apperr.Wrap(err)
	}
	items, matches, err := s.nodeRepository.Search(ctx, catalogtypes.Search{
		Viewer: viewerOf(p), Root: parent, RootReadable: readable, Filter: filter, Only: only, Limit: catalogtypes.TableMaxMatches,
	})
	if err != nil {
		return CatalogTablePage{}, apperr.Wrap(err)
	}
	return CatalogTablePage{Items: items, Total: uint64(len(items)), Limit: uint32(len(items)),
		Truncated: matches > catalogtypes.TableMaxMatches}, nil
}

func (s *TableService) onlyProjects(ctx context.Context, p accesstypes.Principal, filter catalogtypes.TableFilter) ([]uuid.UUID, error) {
	var only []uuid.UUID
	if filter.Activity != "" {
		busy, err := s.activityService.signals(ctx, nil, false, true)
		if err != nil {
			return nil, err
		}
		only = []uuid.UUID{}
		for id, signals := range busy {
			if catalogtypes.Has(catalogtypes.ProcessesOf(signals), filter.Activity) {
				only = append(only, id)
			}
		}
	}
	if filter.Owner == nil {
		return only, nil
	}
	owned, err := s.owners.OwnedProjects(ctx, p.UserID, *filter.Owner)
	if err != nil {
		return nil, err
	}
	if only == nil {
		return append([]uuid.UUID{}, owned...), nil
	}
	return catalogtypes.Intersect(only, owned), nil
}

func (p CatalogTablePage) ReadableNodes() []uuid.UUID {
	var out []uuid.UUID
	for _, n := range p.Items {
		if n.Readable {
			out = append(out, n.Node.ID)
		}
	}
	return out
}

func (p CatalogTablePage) ReadableProjects() []uuid.UUID {
	var out []uuid.UUID
	for _, n := range p.Items {
		if n.Readable && n.Node.Kind == catalogtypes.KindProject {
			out = append(out, n.Node.ID)
		}
	}
	return out
}

type TableService struct {
	nodeRepository  *repositories.NodeRepository
	activityService *ActivityService
	nodesService    *NodesService
	owners          catalogports.OwnersFilterPort
}

func (s *LabelsService) SuggestLabels(ctx context.Context, p accesstypes.Principal, q catalogtypes.LabelQuery) ([]catalogtypes.LabelSuggestion, error) {
	v, err := catalogtypes.ValidateLabelQuery(q)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	prefix := catalogtypes.LikePrefix(v.Q)
	var out []catalogtypes.LabelSuggestion
	if v.Key != nil {
		out, err = s.nodeRepository.LabelValues(ctx, viewerOf(p), p.IsSuperadmin, *v.Key, prefix)
	} else {
		out, err = s.nodeRepository.LabelKeys(ctx, viewerOf(p), p.IsSuperadmin, prefix)
	}
	return out, apperr.Wrap(err)
}

type LabelsService struct {
	nodeRepository *repositories.NodeRepository
}
