package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	deploytypes "svc-registry/internal/feature/deploy/types"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	notifytypes "svc-registry/internal/feature/notify/types"
)

func (s *PollService) emitDrifts(ctx context.Context, obs []deploytypes.Observation) error {
	after := time.Duration(s.cfg.Notify.DriftAfterSecs) * time.Second
	now := time.Now()
	seen := map[triple]bool{}
	currents := map[uuid.UUID][]ingesttypes.Environment{}
	for _, o := range obs {
		if o.ProjectID == nil || o.Service == nil || o.Environment == nil || o.Version == nil {
			continue
		}
		t := triple{*o.ProjectID, *o.Service, *o.Environment}
		envs, ok := currents[t.project]
		if !ok {
			var err error
			if envs, err = s.ingest.Accept.CurrentEnvironments(ctx, t.project); err != nil {
				return err
			}
			currents[t.project] = envs
		}
		cur := currentOf(envs, t)
		if seen[t] || cur == nil || cur.Version == *o.Version {
			continue
		}
		seen[t] = true
		ev, err := s.driftEvent(ctx, t, *cur, now, after)
		if err != nil {
			return err
		}
		if ev == nil {
			continue
		}
		if err := s.notify.Outbox.Emit(ctx, *ev); err != nil {
			return err
		}
	}
	return nil
}

func (s *PollService) resolveDrifts(ctx context.Context) error {
	eps, err := s.notify.Outbox.OpenEpisodes(ctx, notifytypes.EventDeploymentDrift)
	if err != nil {
		return err
	}
	for _, ep := range eps {
		var d notifytypes.DriftData
		if json.Unmarshal(ep.Data, &d) != nil {
			continue
		}
		ev, err := s.resolvedEvent(ctx, triple{ep.NodeID, d.Service, d.Environment}, d.Since)
		if err != nil {
			return err
		}
		if ev == nil {
			continue
		}
		ev.Episode = ep.Key
		if err := s.notify.Outbox.Recover(ctx, *ev); err != nil {
			return err
		}
	}
	return nil
}

func (s *PollService) resolvedEvent(ctx context.Context, t triple, since string) (*notifytypes.Event, error) {
	envs, err := s.ingest.Accept.CurrentEnvironments(ctx, t.project)
	if err != nil {
		return nil, err
	}
	cur := currentOf(envs, t)
	if cur == nil {
		return nil, nil
	}
	rows, err := s.workloadRepository.ForPair(ctx, t.project, t.service, t.environment)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if deploytypes.Drifts(cur.Version, r.Observed) {
			return nil, nil
		}
	}
	env := t.environment
	return &notifytypes.Event{Type: notifytypes.EventDriftResolved, NodeID: t.project, Environment: &env, OccurredAt: time.Now(),
		Data: notifytypes.DriftResolvedData{Service: t.service, Environment: t.environment, Version: cur.Version,
			DeploymentID: cur.DeploymentID, Since: since}}, nil
}

func currentOf(envs []ingesttypes.Environment, t triple) *ingesttypes.Environment {
	for i := range envs {
		if envs[i].Service == t.service && envs[i].Environment == t.environment {
			return &envs[i]
		}
	}
	return nil
}

func (s *PollService) driftEvent(ctx context.Context, t triple, cur ingesttypes.Environment, now time.Time, after time.Duration) (*notifytypes.Event, error) {
	rows, err := s.workloadRepository.ForPair(ctx, t.project, t.service, t.environment)
	if err != nil {
		return nil, err
	}
	since, err := time.Parse(time.RFC3339, cur.OccurredAt)
	if err != nil {
		return nil, nil
	}
	var observed []notifytypes.ObservedVersion
	for _, r := range rows {
		if !deploytypes.Drifts(cur.Version, r.Observed) {
			continue
		}
		since = later(since, r.VersionSince)
		if len(observed) < notifytypes.MaxObserved {
			observed = append(observed, notifytypes.ObservedVersion{Cluster: r.Observed.Cluster, Namespace: r.Observed.Namespace,
				Version: *r.Observed.Version})
		}
	}
	if len(observed) == 0 || now.Sub(since) < after {
		return nil, nil
	}
	env := t.environment
	return &notifytypes.Event{Type: notifytypes.EventDeploymentDrift, NodeID: t.project, Environment: &env,
		DedupeKey:  notifytypes.DriftKey(t.project, t.service, t.environment, cur.DeploymentID, observed),
		Episode:    notifytypes.DriftEpisode(t.project, t.service, t.environment),
		OccurredAt: now,
		Data: notifytypes.DriftData{Service: t.service, Environment: t.environment, Version: cur.Version,
			DeploymentID: cur.DeploymentID, Observed: observed, Since: since.UTC().Format(time.RFC3339)}}, nil
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
