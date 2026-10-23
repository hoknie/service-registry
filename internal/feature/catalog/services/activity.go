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

func (s *ActivityService) NodeActivity(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (catalogtypes.Activity, error) {
	sc, err := s.access.Scope(ctx, p, id)
	if err != nil {
		return catalogtypes.Activity{}, err
	}
	project := sc.Node.Kind == catalogtypes.KindProject
	out := catalogtypes.Activity{Project: project, Processes: []catalogtypes.Process{}}
	if !sc.Allows(catalogtypes.PermRead) {
		return out, nil
	}
	if project {
		signals, err := s.signals(ctx, []uuid.UUID{id}, true, false)
		if err != nil {
			return catalogtypes.Activity{}, apperr.Wrap(err)
		}
		out.Processes = catalogtypes.ProcessesOf(signals[id])
		return out, nil
	}
	_, summaries, err := s.activities(ctx, p, nil, []uuid.UUID{id})
	if err != nil {
		return catalogtypes.Activity{}, err
	}
	out.Summary = summaries[id]
	var own []catalogtypes.ProcessSignals
	for _, src := range s.sources {
		if cs, ok := src.(catalogports.ContainerActivitySourcePort); ok {
			found, err := cs.ContainerSignals(ctx, id)
			if err != nil {
				return catalogtypes.Activity{}, apperr.Wrap(err)
			}
			own = append(own, found...)
		}
	}
	out.Processes = catalogtypes.ProcessesOf(own)
	return out, nil
}

func (s *ActivityService) activities(ctx context.Context, p accesstypes.Principal, projects, containers []uuid.UUID) (map[uuid.UUID][]catalogtypes.Process, map[uuid.UUID]catalogtypes.Summary, error) {
	processes := map[uuid.UUID][]catalogtypes.Process{}
	summaries := map[uuid.UUID]catalogtypes.Summary{}
	signals, err := s.signals(ctx, projects, false, false)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	for _, id := range projects {
		processes[id] = catalogtypes.ProcessesOf(signals[id])
	}
	if len(containers) == 0 {
		return processes, summaries, nil
	}
	busy, err := s.signals(ctx, nil, false, true)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	ids := make([]uuid.UUID, 0, len(busy))
	busyProcesses := make(map[uuid.UUID][]catalogtypes.Process, len(busy))
	for id, sig := range busy {
		ids = append(ids, id)
		busyProcesses[id] = catalogtypes.ProcessesOf(sig)
	}
	within, err := s.activityRepository.Containers(ctx, viewerOf(p), p.IsSuperadmin, ids, containers)
	if err != nil {
		return nil, nil, apperr.Wrap(err)
	}
	for _, c := range containers {
		summaries[c] = catalogtypes.Summarize(within[c], busyProcesses)
	}
	return processes, summaries, nil
}

func (s *ActivityService) signals(ctx context.Context, projects []uuid.UUID, pending, busy bool) (map[uuid.UUID][]catalogtypes.ProcessSignals, error) {
	out := map[uuid.UUID][]catalogtypes.ProcessSignals{}
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

type ActivityService struct {
	activityRepository *repositories.ActivityRepository
	sources            []catalogports.ActivitySourcePort
	access             *AccessService
}
