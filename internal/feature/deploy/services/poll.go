package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	deployports "svc-registry/internal/feature/deploy/ports"
	"svc-registry/internal/feature/deploy/repositories"
	deploytypes "svc-registry/internal/feature/deploy/types"
	ingestservices "svc-registry/internal/feature/ingest/services"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

func (s *PollService) ClaimClusterPolls(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	ids, err := s.clusterRepository.Claim(ctx, limit, leaseSecs)
	return ids, apperr.Wrap(err)
}

func (s *PollService) ExtendClusterPoll(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return apperr.Wrap(s.clusterRepository.Extend(ctx, id, leaseSecs))
}

type triple struct {
	project              uuid.UUID
	service, environment string
}

func (s *PollService) RunClusterPoll(ctx context.Context, id uuid.UUID) error {
	c, err := s.clusterRepository.Get(ctx, id)
	if err != nil || c == nil {
		return apperr.Wrap(err)
	}
	finish := func(f *deploytypes.Failure) error {
		if ctx.Err() != nil {
			return nil
		}
		return apperr.Wrap(s.clusterRepository.Finish(context.WithoutCancel(ctx), id, f))
	}
	client, err := s.clustersService.clusterClient(ctx, *c)
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
	previous, err := s.workloadRepository.Previous(ctx, id)
	if err != nil {
		return apperr.Wrap(err)
	}
	run := &pollRun{poll: s, cluster: *c, client: client, previous: previous,
		projects: map[string]*deploytypes.ProjectRef{}, currents: map[triple]*string{},
		now:      time.Now().UTC().Truncate(time.Second),
		confirm:  time.Duration(s.cfg.K8s.HistoryConfirmSecs) * time.Second,
		interval: time.Duration(c.IntervalSecs) * time.Second}
	for _, w := range workloads {
		if err := run.observe(ctx, w); err != nil {
			return err
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	if err := s.saveObservations(ctx, id, run.obs, run.deployments); err != nil {
		return apperr.Wrap(err)
	}
	return finish(nil)
}

type pollRun struct {
	poll              *PollService
	cluster           deploytypes.Cluster
	client            deployports.ClusterClientPort
	previous          map[string]deploytypes.Previous
	projects          map[string]*deploytypes.ProjectRef
	currents          map[triple]*string
	now               time.Time
	confirm, interval time.Duration
	obs               []deploytypes.Observation
	deployments       []deploytypes.ClusterDeployment
}

func (r *pollRun) resolve(ctx context.Context) deploytypes.Resolver {
	return func(ref string) (*deploytypes.ProjectRef, error) {
		if p, ok := r.projects[ref]; ok {
			return p, nil
		}
		p, err := r.poll.workloadRepository.Project(ctx, ref)
		if err == nil {
			r.projects[ref] = p
		}
		return p, err
	}
}

func (r *pollRun) current(ctx context.Context, t triple) (*string, error) {
	if v, ok := r.currents[t]; ok {
		return v, nil
	}
	v, err := r.poll.workloadRepository.Current(ctx, t.project, t.service, t.environment)
	if err == nil {
		r.currents[t] = v
	}
	return v, err
}

func (r *pollRun) observe(ctx context.Context, w deploytypes.Workload) error {
	c, now := r.cluster, r.now
	m, err := deploytypes.MatchWorkload(w, c.Environment, c.Rules, r.resolve(ctx))
	if err != nil {
		return apperr.Wrap(err)
	}
	if m.Project != nil && !m.Project.Observe {
		return nil
	}
	prev := r.previous[w.UID]
	o := deploytypes.Observation{UID: w.UID, Kind: w.Kind, Namespace: w.Namespace, Name: w.Name, Annotation: m.Annotation, Version: m.Version}
	if m.Project == nil {
		reason := m.Reason
		o.Reason = &reason
		o.State, o.Images = deploytypes.ObserveState(w, nil, nil, prev.State, now, r.interval)
		r.obs = append(r.obs, o)
		return nil
	}
	project := m.Project.ID
	o.ProjectID, o.Service, o.Environment, o.Branch = &project, &m.Service, &m.Environment, m.Branch
	pods, jobs := observeDetails(ctx, r.client, w)
	o.State, o.Images = deploytypes.ObserveState(w, pods, jobs, prev.State, now, r.interval)
	t := triple{project, m.Service, m.Environment}
	cur, err := r.current(ctx, t)
	if err != nil {
		return apperr.Wrap(err)
	}
	var waiting *deploytypes.Pending
	if prev.PendingVersion != nil && prev.PendingSince != nil {
		waiting = &deploytypes.Pending{Version: *prev.PendingVersion, Since: *prev.PendingSince}
	}
	pending := deploytypes.NextPending(cur, m.Version, waiting, now)
	if pending != nil && pending.Since.Equal(now) {
		o.BranchActivity = &now
	}
	if deploytypes.Due(pending, now, r.confirm) {
		r.deployments = append(r.deployments, deploytypes.ClusterDeployment{
			ID: uuid.Must(uuid.NewV7()), ProjectID: project, Service: m.Service, Environment: m.Environment,
			Version: pending.Version, Branch: m.Branch, CommitSHA: m.Commit, Cluster: c.Name, Namespace: w.Namespace,
			OccurredAt: pending.Since,
		})
		v := pending.Version
		r.currents[t] = &v
		pending = nil
	}
	if pending != nil {
		o.PendingVersion, o.PendingSince = &pending.Version, &pending.Since
	}
	r.obs = append(r.obs, o)
	return nil
}

func observeDetails(ctx context.Context, client deployports.ClusterClientPort, w deploytypes.Workload) ([]deploytypes.Pod, []deploytypes.Job) {
	if w.Kind == deploytypes.KindCronJob {
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

func (s *PollService) PruneWorkloads(ctx context.Context) (int64, error) {
	n, err := s.workloadRepository.Prune(ctx, s.cfg.K8s.WorkloadRetentionDays)
	return n, apperr.Wrap(err)
}

func (s *PollService) saveObservations(ctx context.Context, clusterID uuid.UUID, obs []deploytypes.Observation, deployments []deploytypes.ClusterDeployment) error {
	return repositories.Err(s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.workloadRepository.Save(ctx, clusterID, obs); err != nil {
			return err
		}
		for _, o := range obs {
			if o.ProjectID != nil && o.Branch != nil {
				if err := s.catalog.BranchRecords.RecordClusterBranch(ctx, *o.ProjectID, *o.Branch, nil, o.BranchActivity); err != nil {
					return err
				}
			}
		}
		for _, d := range deployments {
			cluster, namespace := d.Cluster, d.Namespace
			if _, err := s.ingest.Accept.RecordDeployment(ctx, ingesttypes.DeploymentRecord{
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

type PollService struct {
	catalog            *catalogservices.CatalogServices
	cfg                *config.Config
	clusterRepository  *repositories.ClusterRepository
	db                 *pgtx.DB
	ingest             *ingestservices.IngestServices
	workloadRepository *repositories.WorkloadRepository
	clustersService    *ClustersService
}
