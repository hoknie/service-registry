package service

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/deploy"
	"svc-registry/internal/feature/ingest"
	"svc-registry/internal/platform/apperr"
)

type EnvironmentState struct {
	Service     string
	Environment string
	Current     *ingest.Environment
	Observed    []deploy.Observed
	Drift       bool
}

func (s *Service) ListEnvironments(ctx context.Context, p access.Principal, id uuid.UUID) ([]EnvironmentState, error) {
	if err := s.catalog.AuthorizeProject(ctx, p, catalog.PermRead, id); err != nil {
		return nil, err
	}
	currents, err := s.ingest.CurrentEnvironments(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	observed, err := s.workloads.ForProject(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	directory, err := s.environments.List(ctx)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	type pair struct{ service, environment string }
	byPair := map[pair]*EnvironmentState{}
	var out []*EnvironmentState
	get := func(service, environment string) *EnvironmentState {
		k := pair{service, environment}
		if e, ok := byPair[k]; ok {
			return e
		}
		e := &EnvironmentState{Service: service, Environment: environment, Observed: []deploy.Observed{}}
		byPair[k] = e
		out = append(out, e)
		return e
	}
	for i := range currents {
		get(currents[i].Service, currents[i].Environment).Current = &currents[i]
	}
	for _, o := range observed {
		e := get(o.Service, o.Environment)
		e.Observed = append(e.Observed, o)
		if e.Current != nil && !o.Gone && o.Version != nil && *o.Version != e.Current.Version {
			e.Drift = true
		}
	}
	order := deploy.EnvironmentOrder(directory)
	slices.SortFunc(out, func(a, b *EnvironmentState) int {
		return cmp.Or(strings.Compare(a.Service, b.Service), order(a.Environment, b.Environment))
	})
	items := make([]EnvironmentState, 0, len(out))
	for _, e := range out {
		items = append(items, *e)
	}
	return items, nil
}
