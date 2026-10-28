package app

import (
	"net/http"
	"os"
	"time"

	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/metrics"
)

type instrumented interface {
	Instrument(wrap func(http.RoundTripper) http.RoundTripper)
}

func newMetrics(cfg config.Config) *metrics.Metrics {
	if !cfg.Metrics.Enabled {
		return nil
	}
	return metrics.New(Version, []string{string(ingesttypes.TypeServiceDeployed)})
}

func instrument[T any](m *metrics.Metrics, target string, client T) T {
	if c, ok := any(client).(instrumented); ok && m != nil {
		c.Instrument(func(rt http.RoundTripper) http.RoundTripper { return m.RoundTripper(target, rt) })
	}
	return client
}

func (a *App) newSampler(extra []metrics.Source) *metrics.Sampler {
	if a.Metrics == nil {
		return nil
	}
	pool := a.DB.Pool()
	sources := []metrics.Source{
		metrics.RuntimeSource(),
		metrics.PoolSource(func() metrics.PoolStat {
			s := pool.Stat()
			return metrics.PoolStat{Acquired: s.AcquiredConns(), Idle: s.IdleConns(), Constructing: s.ConstructingConns(),
				Max: s.MaxConns(), Acquires: s.AcquireCount(), EmptyAcquires: s.EmptyAcquireCount(), AcquireWait: s.AcquireDuration()}
		}),
		metrics.SampleSource("catalog", a.Catalog.System.MetricSamples, a.Access.Users.MetricSamples, a.Ownership.Ports.MetricSamples,
			a.Ingest.Accept.MetricSamples, a.Deploy.Ports.MetricSamples, a.RepoSync.Ports.MetricSamples, a.Links.Projects.MetricSamples,
			a.Notify.Outbox.MetricSamples, a.Topology.Records.MetricSamples, a.Knowledge.Search.MetricSamples, a.gatewaySamples),
		metrics.SampleSource("jobs", a.Deploy.Ports.QueueSamples, a.RepoSync.Ports.QueueSamples, a.Links.Projects.QueueSamples,
			a.Notify.Outbox.QueueSamples, a.Knowledge.Search.QueueSamples),
	}
	return a.Metrics.Sampler(time.Duration(a.Config.Metrics.RefreshSecs)*time.Second, append(sources, extra...)...)
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}
