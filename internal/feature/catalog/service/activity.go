package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) NodeActivity(ctx context.Context, p access.Principal, id uuid.UUID) (catalog.Activity, error) {
	sc, err := s.Scope(ctx, p, id)
	if err != nil {
		return catalog.Activity{}, err
	}
	project := sc.Node.Kind == catalog.KindProject
	out := catalog.Activity{Project: project, Processes: []catalog.Process{}}
	if !sc.Allows(catalog.PermRead) {
		return out, nil
	}
	if project {
		signals, err := s.signals(ctx, []uuid.UUID{id}, true, false)
		if err != nil {
			return catalog.Activity{}, apperr.Wrap(err)
		}
		out.Processes = catalog.ProcessesOf(signals[id])
		return out, nil
	}
	_, summaries, err := s.activities(ctx, p, nil, []uuid.UUID{id})
	if err != nil {
		return catalog.Activity{}, err
	}
	out.Summary = summaries[id]
	return out, nil
}

func (s *Service) activities(ctx context.Context, p access.Principal, projects, containers []uuid.UUID) (map[uuid.UUID][]catalog.Process, map[uuid.UUID]catalog.Summary, error) {
	processes := map[uuid.UUID][]catalog.Process{}
	summaries := map[uuid.UUID]catalog.Summary{}
	signals, err := s.signals(ctx, projects, false, false)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	for _, id := range projects {
		processes[id] = catalog.ProcessesOf(signals[id])
	}
	if len(containers) == 0 {
		return processes, summaries, nil
	}
	busy, err := s.signals(ctx, nil, false, true)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	ids := make([]uuid.UUID, 0, len(busy))
	busyProcesses := make(map[uuid.UUID][]catalog.Process, len(busy))
	for id, sig := range busy {
		ids = append(ids, id)
		busyProcesses[id] = catalog.ProcessesOf(sig)
	}
	within, err := s.activity.Containers(ctx, p.UserID, p.IsSuperadmin, ids, containers)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	for _, c := range containers {
		summaries[c] = catalog.Summarize(within[c], busyProcesses)
	}
	return processes, summaries, nil
}

func (s *Service) signals(ctx context.Context, projects []uuid.UUID, pending, busy bool) (map[uuid.UUID][]catalog.ProcessSignals, error) {
	out := map[uuid.UUID][]catalog.ProcessSignals{}
	if !busy && len(projects) == 0 {
		return out, nil
	}
	for _, src := range s.sources {
		found, err := src.ActivitySignals(ctx, projects, pending, busy)
		if err != nil {
			return nil, err
		}
		for id, list := range found {
			out[id] = append(out[id], list...)
		}
	}
	return out, nil
}
