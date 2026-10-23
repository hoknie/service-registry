package services

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/topology/repositories"
	topologytypes "svc-registry/internal/feature/topology/types"
	"svc-registry/internal/platform/apperr"
)

func (s *DependenciesService) project(ctx context.Context, p accesstypes.Principal, perm catalogtypes.Permission, id uuid.UUID) error {
	sc, err := s.catalog.Access.Authorize(ctx, p, perm, id)
	if err != nil {
		return err
	}
	if sc.Node.Kind != catalogtypes.KindProject {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func (s *DependenciesService) ProjectDependencies(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (topologytypes.List, error) {
	if err := s.project(ctx, p, catalogtypes.PermRead, id); err != nil {
		return topologytypes.List{}, err
	}
	list, _, err := s.list(ctx, p, id)
	return list, err
}

func (s *DependenciesService) AddDependency(ctx context.Context, p accesstypes.Principal, id uuid.UUID, target string, note *string) (topologytypes.Item, error) {
	if err := s.project(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return topologytypes.Item{}, err
	}
	ref, err := topologytypes.ParseRef(target)
	note, ok := topologytypes.ValidNote(note)
	if err != nil || !ok {
		return topologytypes.Item{}, topologytypes.InvalidDependency
	}
	targets, err := s.targets(ctx, id, []topologytypes.Ref{ref})
	if err != nil {
		return topologytypes.Item{}, apperr.Wrap(err)
	}
	if len(targets) != 1 || (ref.External == "" && targets[0].ProjectID == nil) {
		return topologytypes.Item{}, topologytypes.InvalidDependency
	}
	t := targets[0]
	if t.ProjectID != nil {
		readable, err := s.catalog.Access.ReadableProjects(ctx, p, []uuid.UUID{*t.ProjectID})
		if err != nil {
			return topologytypes.Item{}, err
		}
		if !readable[*t.ProjectID] {
			return topologytypes.Item{}, topologytypes.InvalidDependency
		}
	}
	if _, err := s.dependencyRepository.InsertManual(ctx, id, t, note); err != nil {
		return topologytypes.Item{}, apperr.Wrap(err)
	}
	_, keyed, err := s.list(ctx, p, id)
	if err != nil {
		return topologytypes.Item{}, err
	}
	key := topologytypes.Edge{TargetProjectID: t.ProjectID, TargetExternal: t.External, TargetRef: t.Ref}.TargetKey()
	return keyed[key], nil
}

func (s *DependenciesService) DeleteDependency(ctx context.Context, p accesstypes.Principal, id, dependency uuid.UUID) error {
	if err := s.project(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return err
	}
	return apperr.Wrap(s.dependencyRepository.DeleteManual(ctx, id, dependency))
}

func (s *DependenciesService) list(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (topologytypes.List, map[string]topologytypes.Item, error) {
	edges, err := s.dependencyRepository.Touching(ctx, []uuid.UUID{id}, false)
	if err != nil {
		return topologytypes.List{}, nil, apperr.Wrap(err)
	}
	var ids []uuid.UUID
	for _, e := range edges {
		if e.ProjectID == id && e.TargetProjectID != nil {
			ids = append(ids, *e.TargetProjectID)
		} else if e.ProjectID != id {
			ids = append(ids, e.ProjectID)
		}
	}
	known, err := s.describe(ctx, p, ids)
	if err != nil {
		return topologytypes.List{}, nil, err
	}
	keyed := map[string]topologytypes.Item{}
	var order []string
	dependents := map[uuid.UUID]*topologytypes.Dependent{}
	hidden := map[uuid.UUID]bool{}
	for _, e := range edges {
		if e.ProjectID != id {
			if ref, ok := known[e.ProjectID]; ok {
				if dependents[e.ProjectID] == nil {
					dependents[e.ProjectID] = &topologytypes.Dependent{Project: ref}
				}
				dependents[e.ProjectID].Sources = append(dependents[e.ProjectID].Sources, topologytypes.SourceOf(e))
			} else {
				hidden[e.ProjectID] = true
			}
			continue
		}
		key := e.TargetKey()
		item, ok := keyed[key]
		if !ok {
			item.Target = targetOf(e, known)
			order = append(order, key)
		}
		item.Sources = append(item.Sources, topologytypes.SourceOf(e))
		keyed[key] = item
	}
	list := topologytypes.List{DependsOn: []topologytypes.Item{}, Dependents: []topologytypes.Dependent{}, HiddenDependents: len(hidden)}
	for _, k := range order {
		list.DependsOn = append(list.DependsOn, keyed[k])
	}
	topologytypes.SortItems(list.DependsOn)
	for _, d := range dependents {
		list.Dependents = append(list.Dependents, *d)
	}
	slices.SortFunc(list.Dependents, func(a, b topologytypes.Dependent) int { return strings.Compare(a.Project.Path, b.Project.Path) })
	return list, keyed, nil
}

func targetOf(e topologytypes.Edge, known map[uuid.UUID]topologytypes.ProjectRef) topologytypes.Target {
	switch {
	case e.TargetProjectID != nil:
		if ref, ok := known[*e.TargetProjectID]; ok {
			return topologytypes.Target{Type: topologytypes.TargetProject, Project: &ref}
		}
		return topologytypes.Target{Type: topologytypes.TargetHidden}
	case e.TargetExternal != nil:
		return topologytypes.Target{Type: topologytypes.TargetExternal, External: *e.TargetExternal}
	}
	return topologytypes.Target{Type: topologytypes.TargetUnresolved, Ref: e.TargetRef}
}

func (s *DependenciesService) describe(ctx context.Context, p accesstypes.Principal, ids []uuid.UUID) (map[uuid.UUID]topologytypes.ProjectRef, error) {
	readable, err := s.catalog.Access.ReadableProjects(ctx, p, ids)
	if err != nil {
		return nil, err
	}
	visible := make([]uuid.UUID, 0, len(readable))
	for id := range readable {
		visible = append(visible, id)
	}
	refs, err := s.catalog.Access.DescribeRoots(ctx, visible)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]topologytypes.ProjectRef, len(refs))
	for id, r := range refs {
		if r.Path != nil && r.Name != nil {
			out[id] = topologytypes.ProjectRef{ID: id, Path: *r.Path, Name: *r.Name}
		}
	}
	return out, nil
}

type DependenciesService struct {
	resolver
	dependencyRepository *repositories.DependencyRepository
}
