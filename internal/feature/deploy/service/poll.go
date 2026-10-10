package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/feature/deploy"
	"svc-registry/internal/feature/deploy/internal/repository"
	"svc-registry/internal/feature/ingest"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) ClaimClusterPolls(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	ids, err := s.clusters.Claim(ctx, limit, leaseSecs)
	return ids, apperr.Wrap(err)
}

func (s *Service) ExtendClusterPoll(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return apperr.Wrap(s.clusters.Extend(ctx, id, leaseSecs))
}

type triple struct {
	project              uuid.UUID
	service, environment string
}

func (s *Service) RunClusterPoll(ctx context.Context, id uuid.UUID) error {
	c, err := s.clusters.Get(ctx, id)
	if err != nil || c == nil {
		return apperr.Wrap(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	finish := func(f *deploy.Failure) error {
		if ctx.Err() != nil {
			return nil
		}
		return apperr.Wrap(s.clusters.Finish(context.WithoutCancel(ctx), id, f))
	}
	client, err := s.clusterClient(ctx, *c)
	if f := asFailure(err); f != nil {
		return finish(f)
	} else if err != nil {
		return err
	}
	workloads, err := client.Workloads(ctx, c.Namespaces)
	if f := asFailure(err); f != nil {
		return finish(f)
	} else if err != nil {
		return err
	}
	previous, err := s.workloads.Previous(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	projects := map[string]*deploy.ProjectRef{}
	resolve := func(ref string) (*deploy.ProjectRef, error) {
		if p, ok := projects[ref]; ok {
			return p, nil
		}
		p, err := s.workloads.Project(ctx, ref)
		if err == nil {
			projects[ref] = p
		}
		return p, err
	}
	currents := map[triple]*string{}
	current := func(t triple) (*string, error) {
		if v, ok := currents[t]; ok {
			return v, nil
		}
		v, err := s.workloads.Current(ctx, t.project, t.service, t.environment)
		if err == nil {
			currents[t] = v
		}
		return v, err
	}
	cfg := s.cfg.K8s
	confirm := time.Duration(cfg.HistoryConfirmSecs) * time.Second
	interval := time.Duration(c.IntervalSecs) * time.Second
	var obs []deploy.Observation
	var deployments []deploy.ClusterDeployment
	for _, w := range workloads {
		m, err := deploy.MatchWorkload(w, c.Environment, c.Rules, resolve)
		if err != nil {
			return apperr.Wrap(err)
		}
		if m.Project != nil && !m.Project.Observe {
			continue
		}
		prev := previous[w.UID]
		o := deploy.Observation{UID: w.UID, Kind: w.Kind, Namespace: w.Namespace, Name: w.Name, Annotation: m.Annotation, Version: m.Version}
		if m.Project == nil {
			reason := m.Reason
			o.Reason = &reason
			o.State, o.Images = deploy.ObserveState(w, nil, nil, prev.State, now, interval)
			obs = append(obs, o)
			continue
		}
		project := m.Project.ID
		o.ProjectID, o.Service, o.Environment, o.Branch = &project, &m.Service, &m.Environment, m.Branch
		pods, jobs := observeDetails(ctx, client, w)
		o.State, o.Images = deploy.ObserveState(w, pods, jobs, prev.State, now, interval)
		t := triple{project, m.Service, m.Environment}
		cur, err := current(t)
		if err != nil {
			return apperr.Wrap(err)
		}
		var waiting *deploy.Pending
		if prev.PendingVersion != nil && prev.PendingSince != nil {
			waiting = &deploy.Pending{Version: *prev.PendingVersion, Since: *prev.PendingSince}
		}
		pending := deploy.NextPending(cur, m.Version, waiting, now)
		if pending != nil && pending.Since.Equal(now) {
			o.BranchActivity = &now
		}
		if deploy.Due(pending, now, confirm) {
			deployments = append(deployments, deploy.ClusterDeployment{
				ID: uuid.Must(uuid.NewV7()), ProjectID: project, Service: m.Service, Environment: m.Environment,
				Version: pending.Version, Branch: m.Branch, CommitSHA: m.Commit, Cluster: c.Name, Namespace: w.Namespace,
				OccurredAt: pending.Since,
			})
			v := pending.Version
			currents[t] = &v
			pending = nil
		}
		if pending != nil {
			o.PendingVersion, o.PendingSince = &pending.Version, &pending.Since
		}
		obs = append(obs, o)
	}
	if ctx.Err() != nil {
		return nil
	}
	if err := s.saveObservations(ctx, id, obs, deployments); err != nil {
		return apperr.Wrap(err)
	}
	return finish(nil)
}

func observeDetails(ctx context.Context, client deploy.ClusterClient, w deploy.Workload) ([]deploy.Pod, []deploy.Job) {
	if w.Kind == deploy.KindCronJob {
		jobs, err := client.Jobs(ctx, w.Namespace, w.UID)
		if err != nil {
			slog.Debug("cluster poll: jobs not read", "workload", w.Namespace+"/"+w.Name, "error", err)
		}
		return nil, jobs
	}
	if w.Selector == "" {
		return nil, nil
	}
	pods, err := client.Pods(ctx, w.Namespace, w.Selector)
	if err != nil {
		slog.Debug("cluster poll: pods not read", "workload", w.Namespace+"/"+w.Name, "error", err)
	}
	return pods, nil
}

func (s *Service) PruneWorkloads(ctx context.Context) (int64, error) {
	n, err := s.workloads.Prune(ctx, s.cfg.K8s.WorkloadRetentionDays)
	return n, apperr.Wrap(err)
}

func (s *Service) saveObservations(ctx context.Context, clusterID uuid.UUID, obs []deploy.Observation, deployments []deploy.ClusterDeployment) error {
	return repository.Err(s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.workloads.Save(ctx, clusterID, obs); err != nil {
			return err
		}
		for _, o := range obs {
			if o.ProjectID != nil && o.Branch != nil {
				if err := s.catalog.RecordClusterBranch(ctx, *o.ProjectID, *o.Branch, nil, o.BranchActivity); err != nil {
					return err
				}
			}
		}
		for _, d := range deployments {
			cluster, namespace := d.Cluster, d.Namespace
			if _, err := s.ingest.RecordDeployment(ctx, ingest.DeploymentRecord{
				ID: d.ID, ProjectID: d.ProjectID, Source: "cluster", Service: d.Service, Environment: d.Environment,
				Version: d.Version, CommitSHA: d.CommitSHA, Branch: d.Branch, Cluster: &cluster, Namespace: &namespace,
				OccurredAt: d.OccurredAt,
			}); err != nil {
				return err
			}
		}
		return nil
	}))
}
