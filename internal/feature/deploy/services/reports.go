package services

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"svc-registry/internal/feature/deploy/repositories"
	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
)

const reportLeaseSecs = 300

type Agent struct{ clusterID uuid.UUID }

func (s *ReportsService) AuthenticateAgent(ctx context.Context, clusterID uuid.UUID, bearer *string) (Agent, error) {
	if bearer == nil {
		return Agent{}, apperr.New(apperr.InvalidAgentToken)
	}
	hash, ok := auth.ParseAgentToken(*bearer)
	if !ok {
		return Agent{}, apperr.New(apperr.InvalidAgentToken)
	}
	valid, err := s.agentTokenRepository.Verify(ctx, clusterID, hash)
	if err != nil {
		return Agent{}, apperr.Wrap(err)
	}
	if !valid {
		return Agent{}, apperr.New(apperr.InvalidAgentToken)
	}
	return Agent{clusterID: clusterID}, nil
}

func (s *ReportsService) AgentConfig(ctx context.Context, a Agent) (deploytypes.AgentConfig, error) {
	c, err := s.cluster(ctx, a)
	if err != nil {
		return deploytypes.AgentConfig{}, err
	}
	return deploytypes.ConfigOf(c), nil
}

func (s *ReportsService) AcceptReport(ctx context.Context, a Agent, r deploytypes.Report) (deploytypes.ReportAnswer, error) {
	if err := deploytypes.ValidateReport(r); err != nil {
		return deploytypes.ReportAnswer{}, apperr.Wrap(err)
	}
	lease, err := s.clusterRepository.LeaseReport(ctx, a.clusterID, reportLeaseSecs)
	switch {
	case err != nil:
		return deploytypes.ReportAnswer{}, apperr.Wrap(err)
	case lease.Leased:
	case lease.Disabled:
		return deploytypes.ReportAnswer{}, deploytypes.ConflictClusterDisabled
	case lease.Busy:
		return deploytypes.ReportAnswer{}, deploytypes.ConflictReportInProgress
	default:
		return deploytypes.ReportAnswer{}, &apperr.Error{Kind: apperr.IngestRateLimited, RetryAfterSecs: uint64(max(lease.RetryAfter, 1))}
	}
	answer, err := s.process(ctx, a, r)
	if err != nil {
		_ = s.clusterRepository.Release(context.WithoutCancel(ctx), a.clusterID)
		return deploytypes.ReportAnswer{}, apperr.Wrap(err)
	}
	return answer, nil
}

func (s *ReportsService) process(ctx context.Context, a Agent, r deploytypes.Report) (deploytypes.ReportAnswer, error) {
	c, err := s.cluster(ctx, a)
	if err != nil {
		return deploytypes.ReportAnswer{}, err
	}
	summary := deploytypes.SummaryOf(r)
	var counts deploytypes.ReportCounts
	if r.Error == nil {
		run, f, err := s.poll.observeCluster(ctx, c, newSnapshot(r))
		if err != nil {
			return deploytypes.ReportAnswer{}, err
		}
		if f != nil {
			summary.Error = &deploytypes.ReportError{Code: string(f.Code), Message: f.Message}
		} else {
			counts.Workloads = len(r.Workloads)
			counts.Matched, counts.Unmatched = run.counts()
		}
	}
	if err := s.clusterRepository.FinishReport(context.WithoutCancel(ctx), c.ID, r.AgentVersion, summary); err != nil {
		return deploytypes.ReportAnswer{}, err
	}
	return deploytypes.ReportAnswer{AgentConfig: deploytypes.ConfigOf(c), Accepted: counts}, nil
}

func (s *ReportsService) cluster(ctx context.Context, a Agent) (deploytypes.Cluster, error) {
	c, err := s.clusterRepository.Get(ctx, a.clusterID)
	if err != nil {
		return deploytypes.Cluster{}, apperr.Wrap(err)
	}
	if c == nil || c.Mode != deploytypes.ModeAgent {
		return deploytypes.Cluster{}, apperr.New(apperr.InvalidAgentToken)
	}
	return *c, nil
}

var errNotReported = errors.New("not in the agent report")

type snapshot struct {
	report deploytypes.Report
	pods   map[string]*[]deploytypes.ReportPod
	jobs   map[string]*[]deploytypes.ReportJob
}

func newSnapshot(r deploytypes.Report) *snapshot {
	s := &snapshot{report: r, pods: map[string]*[]deploytypes.ReportPod{}, jobs: map[string]*[]deploytypes.ReportJob{}}
	for _, w := range r.Workloads {
		if key := w.Namespace + "/" + w.Selector; s.pods[key] == nil {
			s.pods[key] = w.Pods
		}
		s.jobs[w.UID] = w.Jobs
	}
	return s
}

func (s *snapshot) Version(context.Context) (string, error) {
	if s.report.KubernetesVersion == nil {
		return "", nil
	}
	return *s.report.KubernetesVersion, nil
}

func (s *snapshot) Missing(context.Context, []string) ([]string, error) { return s.report.Missing, nil }

func (s *snapshot) Workloads(context.Context, []string) ([]deploytypes.Workload, error) {
	out := make([]deploytypes.Workload, 0, len(s.report.Workloads))
	for _, w := range s.report.Workloads {
		out = append(out, w.Workload())
	}
	return out, nil
}

func (s *snapshot) Pods(_ context.Context, namespace, selector string) ([]deploytypes.Pod, error) {
	pods := s.pods[namespace+"/"+selector]
	if pods == nil {
		return nil, errNotReported
	}
	return deploytypes.PodsOf(*pods), nil
}

func (s *snapshot) Jobs(_ context.Context, _, cronJobUID string) ([]deploytypes.Job, error) {
	jobs := s.jobs[cronJobUID]
	if jobs == nil {
		return nil, errNotReported
	}
	return deploytypes.JobsOf(*jobs), nil
}

type ReportsService struct {
	agentTokenRepository *repositories.AgentTokenRepository
	clusterRepository    *repositories.ClusterRepository
	poll                 *PollService
}
