package service

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
	"svc-registry/internal/deploy"
	"svc-registry/internal/ingest"
)

func authorizeRecords(ctx context.Context, state *State, p Principal, id uuid.UUID) error {
	s, err := Authorize(ctx, state, p, catalog.PermRead, id)
	if err != nil {
		return err
	}
	if s.Node.Kind != catalog.KindProject {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func ListEvents(ctx context.Context, state *State, p Principal, id uuid.UUID, f ingest.EventFilter, q access.PageQuery) (access.Page[ingest.Event], error) {
	if err := authorizeRecords(ctx, state, p, id); err != nil {
		return access.Page[ingest.Event]{}, err
	}
	page, err := access.ValidatePage(q)
	if err != nil {
		return access.Page[ingest.Event]{}, apperr.Wrap(err)
	}
	out, err := state.Events.List(ctx, id, f, page)
	return out, apperr.Wrap(err)
}

func ListDeployments(ctx context.Context, state *State, p Principal, id uuid.UUID, f ingest.DeploymentFilter, q access.PageQuery) (access.Page[ingest.Deployment], error) {
	if err := authorizeRecords(ctx, state, p, id); err != nil {
		return access.Page[ingest.Deployment]{}, err
	}
	page, err := access.ValidatePage(q)
	if err != nil {
		return access.Page[ingest.Deployment]{}, apperr.Wrap(err)
	}
	out, err := state.Deployments.List(ctx, id, f, page)
	return out, apperr.Wrap(err)
}

type EnvironmentState struct {
	Service     string
	Environment string
	Current     *ingest.Environment
	Observed    []deploy.Observed
	Drift       bool
}

func ListEnvironments(ctx context.Context, state *State, p Principal, id uuid.UUID) ([]EnvironmentState, error) {
	if err := authorizeRecords(ctx, state, p, id); err != nil {
		return nil, err
	}
	currents, err := state.Deployments.Environments(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	observed, err := state.Workloads.ForProject(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	directory, err := state.Environments.List(ctx)
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
