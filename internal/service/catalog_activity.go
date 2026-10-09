package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/apperr"
	"svc-registry/internal/catalog"
)

func NodeActivity(ctx context.Context, state *State, p Principal, id uuid.UUID) (catalog.Activity, error) {
	s, err := Scope(ctx, state, p, id)
	if err != nil {
		return catalog.Activity{}, err
	}
	project := s.Node.Kind == catalog.KindProject
	out := catalog.Activity{Project: project, Processes: []catalog.Process{}}
	if !s.Allows(catalog.PermRead) {
		return out, nil
	}
	if project {
		want := activityWant(state)
		want.Pending = true
		signals, err := state.Activity.Signals(ctx, []uuid.UUID{id}, want)
		if err != nil {
			return catalog.Activity{}, apperr.Wrap(err)
		}
		out.Processes = catalog.ProcessesOf(signals[id])
		return out, nil
	}
	_, summaries, err := activities(ctx, state, p, nil, []uuid.UUID{id})
	if err != nil {
		return catalog.Activity{}, err
	}
	out.Summary = summaries[id]
	return out, nil
}

func activityWant(state *State) catalog.ActivityWant {
	want := catalog.ActivityWant{Indexing: state.Embedder != nil || state.ExternalIndex != nil}
	if state.Embedder != nil {
		want.Model = state.Embedder.Model()
	}
	return want
}

func activities(ctx context.Context, state *State, p Principal, projects, containers []uuid.UUID) (map[uuid.UUID][]catalog.Process, map[uuid.UUID]catalog.Summary, error) {
	want := activityWant(state)
	processes := map[uuid.UUID][]catalog.Process{}
	summaries := map[uuid.UUID]catalog.Summary{}
	signals, err := state.Activity.Signals(ctx, projects, want)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	for _, id := range projects {
		processes[id] = catalog.ProcessesOf(signals[id])
	}
	if len(containers) == 0 {
		return processes, summaries, nil
	}
	busy, err := state.Activity.Busy(ctx, want)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	ids := make([]uuid.UUID, 0, len(busy))
	busyProcesses := make(map[uuid.UUID][]catalog.Process, len(busy))
	for id, s := range busy {
		ids = append(ids, id)
		busyProcesses[id] = catalog.ProcessesOf(s)
	}
	within, err := state.Activity.Containers(ctx, p.UserID, p.IsSuperadmin, ids, containers)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	for _, c := range containers {
		summaries[c] = catalog.Summarize(within[c], busyProcesses)
	}
	return processes, summaries, nil
}
