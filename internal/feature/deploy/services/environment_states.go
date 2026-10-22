package services

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/deploy/repositories"
	deploytypes "svc-registry/internal/feature/deploy/types"
	ingestservices "svc-registry/internal/feature/ingest/services"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/internal/platform/apperr"
)

type EnvironmentState struct {
	Service     string
	Environment string
	Current     *ingesttypes.Environment
	Observed    []deploytypes.Observed
	Drift       bool
}

func (s *EnvironmentStatesService) ListEnvironments(ctx context.Context, p accesstypes.Principal, id uuid.UUID) ([]EnvironmentState, error) {
	if err := s.catalog.Branches.AuthorizeProject(ctx, p, catalogtypes.PermRead, id); err != nil {
		return nil, err
	}
	currents, err := s.ingest.Accept.CurrentEnvironments(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	observed, err := s.workloadRepository.ForProject(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	directory, err := s.environmentRepository.List(ctx)
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
		e := &EnvironmentState{Service: service, Environment: environment, Observed: []deploytypes.Observed{}}
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
	order := deploytypes.EnvironmentOrder(directory)
	slices.SortFunc(out, func(a, b *EnvironmentState) int {
		return cmp.Or(strings.Compare(a.Service, b.Service), order(a.Environment, b.Environment))
	})
	items := make([]EnvironmentState, 0, len(out))
	for _, e := range out {
		items = append(items, *e)
	}
	return items, nil
}

type EnvironmentStatesService struct {
	catalog               *catalogservices.CatalogServices
	environmentRepository *repositories.EnvironmentRepository
	ingest                *ingestservices.IngestServices
	workloadRepository    *repositories.WorkloadRepository
}
