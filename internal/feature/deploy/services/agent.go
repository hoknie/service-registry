package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	deployports "svc-registry/internal/feature/deploy/ports"
	deploytypes "svc-registry/internal/feature/deploy/types"
)

type AgentRunner struct {
	cluster  deployports.ClusterClientPort
	registry deployports.RegistryClientPort
	version  string
	health   *AgentHealth
}

func NewAgentRunner(cluster deployports.ClusterClientPort, registry deployports.RegistryClientPort, version string) *AgentRunner {
	return &AgentRunner{cluster: cluster, registry: registry, version: version, health: NewAgentHealth(nil)}
}

func (r *AgentRunner) Health() *AgentHealth {
	return r.health
}

func (r *AgentRunner) Run(ctx context.Context, once bool) error {
	cfg, err := r.config(ctx, once)
	if err != nil {
		return quiet(ctx, err)
	}
	failures := 0
	for {
		if !cfg.Enabled {
			if once {
				return deploytypes.ErrClusterOffline
			}
			slog.Info("agent: the cluster is disabled in the registry")
			r.health.Contact(pause(cfg))
			if !r.wait(ctx, pause(cfg), cfg) {
				return nil
			}
			if cfg, err = r.config(ctx, false); err != nil {
				return quiet(ctx, err)
			}
			continue
		}
		report := r.collect(ctx, cfg)
		answer, err := r.registry.Report(ctx, report)
		if err != nil {
			if once || errors.Is(err, deploytypes.ErrAgentRefused) || ctx.Err() != nil {
				return quiet(ctx, err)
			}
			slog.Warn("agent: report not accepted", "error", err)
			r.health.Beat(pause(cfg))
			if !r.wait(ctx, backoff(failures, pause(cfg), err), cfg) {
				return nil
			}
			failures++
			if cfg, err = r.config(ctx, false); err != nil {
				return quiet(ctx, err)
			}
			continue
		}
		failures = 0
		r.health.Contact(pause(answer.AgentConfig))
		slog.Info("agent: report accepted", "workloads", answer.Accepted.Workloads, "matched", answer.Accepted.Matched,
			"unmatched", answer.Accepted.Unmatched)
		if once {
			return nil
		}
		cfg = answer.AgentConfig
		if !r.wait(ctx, pause(cfg), cfg) {
			return nil
		}
	}
}

func (r *AgentRunner) config(ctx context.Context, once bool) (deploytypes.AgentConfig, error) {
	for attempt := 0; ; attempt++ {
		cfg, err := r.registry.Config(ctx)
		if err == nil || once || errors.Is(err, deploytypes.ErrAgentRefused) || ctx.Err() != nil {
			return cfg, err
		}
		slog.Warn("agent: configuration not read", "error", err)
		r.health.Beat(0)
		if !r.wait(ctx, backoff(attempt, deploytypes.DefaultInterval*time.Second, err), deploytypes.AgentConfig{}) {
			return cfg, ctx.Err()
		}
	}
}

func (r *AgentRunner) collect(ctx context.Context, cfg deploytypes.AgentConfig) deploytypes.Report {
	report := deploytypes.Report{AgentVersion: r.version, Missing: []string{}, Workloads: []deploytypes.ReportWorkload{}}
	fail := func(err error) deploytypes.Report {
		f := asFailure(err)
		if f == nil {
			f = &deploytypes.Failure{Code: deploytypes.FailUnreachable, Message: err.Error()}
		}
		report.Error = &deploytypes.ReportError{Code: string(f.Code), Message: clip(f.Message, deploytypes.MaxReportMessage)}
		report.Workloads = []deploytypes.ReportWorkload{}
		return report
	}
	version, err := r.cluster.Version(ctx)
	if err != nil {
		return fail(err)
	}
	report.KubernetesVersion = &version
	missing, err := r.cluster.Missing(ctx, cfg.Namespaces)
	if err != nil {
		return fail(err)
	}
	report.Missing = append(report.Missing, missing[:min(len(missing), deploytypes.MaxReportMissing)]...)
	workloads, err := r.cluster.Workloads(ctx, cfg.Namespaces)
	if err != nil {
		return fail(err)
	}
	for _, w := range workloads {
		rw := deploytypes.ReportWorkloadOf(w)
		if deploytypes.Candidate(w, cfg.Rules) {
			pods, jobs := r.details(ctx, w)
			rw.Pods, rw.Jobs = pods, jobs
		}
		report.Workloads = append(report.Workloads, rw)
	}
	return report
}

func (r *AgentRunner) details(ctx context.Context, w deploytypes.Workload) (*[]deploytypes.ReportPod, *[]deploytypes.ReportJob) {
	if w.Kind == deploytypes.KindCronJob {
		jobs, err := r.cluster.Jobs(ctx, w.Namespace, w.UID)
		if err != nil {
			slog.Debug("agent: jobs not read", "workload", w.Namespace+"/"+w.Name, "error", err)
			return nil, nil
		}
		return nil, new(deploytypes.ReportJobsOf(jobs))
	}
	if w.Selector == "" {
		return nil, nil
	}
	pods, err := r.cluster.Pods(ctx, w.Namespace, w.Selector)
	if err != nil {
		slog.Debug("agent: pods not read", "workload", w.Namespace+"/"+w.Name, "error", err)
		return nil, nil
	}
	return new(deploytypes.ReportPodsOf(pods)), nil
}

func pause(cfg deploytypes.AgentConfig) time.Duration {
	if p := period(cfg); p > 0 {
		return p
	}
	return deploytypes.DefaultInterval * time.Second
}

func period(cfg deploytypes.AgentConfig) time.Duration {
	return time.Duration(max(cfg.IntervalSecs, cfg.ReportGapSecs, 0)) * time.Second
}

func backoff(attempt int, ceiling time.Duration, err error) time.Duration {
	d := min(time.Second<<min(attempt, 10), ceiling)
	if re, ok := errors.AsType[*deploytypes.RegistryError](err); ok && re.RetryAfter > d {
		d = re.RetryAfter
	}
	return d
}

func (r *AgentRunner) wait(ctx context.Context, d time.Duration, cfg deploytypes.AgentConfig) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		r.health.Beat(period(cfg))
		return true
	}
}

func quiet(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
