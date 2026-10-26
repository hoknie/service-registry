package responses

import (
	"math"
	"time"

	"svc-registry/internal/platform/metrics"
)

type MetricsView struct {
	Instance    string            `json:"instance"`
	Version     string            `json:"version"`
	GoVersion   string            `json:"go_version"`
	StartedAt   *string           `json:"started_at"`
	RefreshSecs int               `json:"refresh_secs"`
	SampledAt   map[string]string `json:"sampled_at"`
	Runtime     map[string]any    `json:"runtime"`
	Families    []MetricFamily    `json:"families"`
}

type MetricFamily struct {
	Name   string `json:"name"`
	Help   string `json:"help"`
	Type   string `json:"type"`
	Series int    `json:"series"`
}

type MetricSeries struct {
	Labels    map[string]string `json:"labels"`
	Value     any               `json:"value,omitempty"`
	Count     any               `json:"count,omitempty"`
	Sum       any               `json:"sum,omitempty"`
	Buckets   []MetricBucket    `json:"buckets,omitempty"`
	Quantiles []MetricQuantile  `json:"quantiles,omitempty"`
}

type MetricBucket struct {
	Le    any `json:"le"`
	Count any `json:"count"`
}

type MetricQuantile struct {
	Quantile any `json:"quantile"`
	Value    any `json:"value"`
}

type MetricSeriesList struct {
	MetricFamily
	Series []MetricSeries `json:"series"`
}

func MetricsViewOf(v metrics.View) MetricsView {
	out := MetricsView{Instance: v.Instance, Version: v.Version, GoVersion: v.GoVersion, RefreshSecs: v.RefreshSecs,
		SampledAt: map[string]string{}, Runtime: map[string]any{}, Families: []MetricFamily{}}
	if !v.StartedAt.IsZero() {
		out.StartedAt = new(stamp(v.StartedAt))
	}
	for src, at := range v.SampledAt {
		out.SampledAt[src] = stamp(at)
	}
	for name, val := range v.Runtime {
		out.Runtime[name] = number(val)
	}
	for _, f := range v.Families {
		out.Families = append(out.Families, MetricFamily{Name: f.Name, Help: f.Help, Type: f.Type, Series: f.Series})
	}
	return out
}

func MetricFamilyOf(f metrics.Family) MetricSeriesList {
	out := MetricSeriesList{MetricFamily: MetricFamily{Name: f.Name, Help: f.Help, Type: f.Type, Series: f.Series}, Series: []MetricSeries{}}
	for _, r := range f.Rows {
		s := MetricSeries{Labels: r.Labels, Value: number(r.Value), Count: number(r.Count), Sum: number(r.Sum)}
		for _, b := range r.Buckets {
			s.Buckets = append(s.Buckets, MetricBucket{Le: finite(b.Le), Count: finite(b.Count)})
		}
		for _, q := range r.Quantiles {
			s.Quantiles = append(s.Quantiles, MetricQuantile{Quantile: finite(q.Quantile), Value: finite(q.Value)})
		}
		out.Series = append(out.Series, s)
	}
	return out
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func number(v *float64) any {
	if v == nil {
		return nil
	}
	return finite(*v)
}

func finite(v float64) any {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "+Inf"
	case math.IsInf(v, -1):
		return "-Inf"
	}
	return v
}
