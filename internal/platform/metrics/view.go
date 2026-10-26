package metrics

import (
	"math"
	"slices"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var runtimeNames = []string{"go_goroutines", "go_threads", "go_sched_gomaxprocs_threads", "go_memstats_heap_alloc_bytes",
	"go_memstats_heap_inuse_bytes", "go_memstats_heap_objects", "go_memstats_sys_bytes", "go_memstats_stack_inuse_bytes",
	"go_memstats_next_gc_bytes", "go_memstats_alloc_bytes_total", "go_gc_duration_seconds_count", "go_gc_duration_seconds_sum",
	"process_cpu_seconds_total", "process_resident_memory_bytes", "process_open_fds", "process_max_fds"}

type View struct {
	Instance    string
	Version     string
	GoVersion   string
	StartedAt   time.Time
	RefreshSecs int
	SampledAt   map[string]time.Time
	Runtime     map[string]*float64
	Families    []FamilyInfo
}

type FamilyInfo struct {
	Name, Help, Type string
	Series           int
}

type Family struct {
	FamilyInfo
	Rows []Series
}

type Series struct {
	Labels    map[string]string
	Value     *float64
	Count     *float64
	Sum       *float64
	Buckets   []Bucket
	Quantiles []Quantile
}

type Bucket struct {
	Le    float64
	Count float64
}

type Quantile struct {
	Quantile, Value float64
}

func (m *Metrics) Families() ([]*dto.MetricFamily, error) {
	return prometheus.Gatherers{m.events, m.snapshot}.Gather()
}

func (m *Metrics) View(instance string, refresh int) (View, error) {
	families, err := m.Families()
	if err != nil {
		return View{}, err
	}
	v := View{Instance: instance, RefreshSecs: refresh, SampledAt: map[string]time.Time{}, Runtime: map[string]*float64{}}
	for _, name := range runtimeNames {
		v.Runtime[name] = nil
	}
	for _, f := range families {
		v.Families = append(v.Families, infoOf(f))
		v.read(f)
	}
	slices.SortFunc(v.Families, func(a, b FamilyInfo) int { return strings.Compare(a.Name, b.Name) })
	return v, nil
}

func (v *View) read(f *dto.MetricFamily) {
	name := f.GetName()
	switch name {
	case "svcr_metrics_sample_timestamp_seconds":
		for _, s := range f.GetMetric() {
			v.SampledAt[labelOf(s, "source")] = unixTime(s.GetGauge().GetValue())
		}
	case "svcr_build_info":
		if len(f.GetMetric()) > 0 {
			v.Version, v.GoVersion = labelOf(f.GetMetric()[0], "version"), labelOf(f.GetMetric()[0], "go_version")
		}
	case "svcr_start_time_seconds":
		if len(f.GetMetric()) > 0 {
			v.StartedAt = unixTime(f.GetMetric()[0].GetGauge().GetValue())
		}
	case "go_gc_duration_seconds":
		if len(f.GetMetric()) > 0 {
			s := f.GetMetric()[0].GetSummary()
			v.Runtime[name+"_count"], v.Runtime[name+"_sum"] = new(float64(s.GetSampleCount())), new(s.GetSampleSum())
		}
	default:
		if _, ok := v.Runtime[name]; ok && len(f.GetMetric()) > 0 {
			v.Runtime[name] = valueOf(f.GetMetric()[0])
		}
	}
}

func (m *Metrics) Family(name string) (*Family, bool, error) {
	families, err := m.Families()
	if err != nil {
		return nil, false, err
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		out := &Family{FamilyInfo: infoOf(f)}
		for _, s := range f.GetMetric() {
			out.Rows = append(out.Rows, seriesOf(f.GetType(), s))
		}
		slices.SortFunc(out.Rows, func(a, b Series) int { return strings.Compare(labelKey(a.Labels), labelKey(b.Labels)) })
		return out, true, nil
	}
	return nil, false, nil
}

func infoOf(f *dto.MetricFamily) FamilyInfo {
	return FamilyInfo{Name: f.GetName(), Help: f.GetHelp(), Type: strings.ToLower(f.GetType().String()), Series: len(f.GetMetric())}
}

func seriesOf(kind dto.MetricType, m *dto.Metric) Series {
	s := Series{Labels: map[string]string{}}
	for _, l := range m.GetLabel() {
		s.Labels[l.GetName()] = l.GetValue()
	}
	switch kind {
	case dto.MetricType_HISTOGRAM:
		h := m.GetHistogram()
		s.Count, s.Sum = new(float64(h.GetSampleCount())), new(h.GetSampleSum())
		for _, b := range h.GetBucket() {
			s.Buckets = append(s.Buckets, Bucket{Le: b.GetUpperBound(), Count: float64(b.GetCumulativeCount())})
		}
		if len(s.Buckets) == 0 || !math.IsInf(s.Buckets[len(s.Buckets)-1].Le, 1) {
			s.Buckets = append(s.Buckets, Bucket{Le: math.Inf(1), Count: float64(h.GetSampleCount())})
		}
	case dto.MetricType_SUMMARY:
		q := m.GetSummary()
		s.Count, s.Sum = new(float64(q.GetSampleCount())), new(q.GetSampleSum())
		for _, x := range q.GetQuantile() {
			s.Quantiles = append(s.Quantiles, Quantile{Quantile: x.GetQuantile(), Value: x.GetValue()})
		}
	default:
		s.Value = valueOf(m)
	}
	return s
}

func valueOf(m *dto.Metric) *float64 {
	switch {
	case m.GetCounter() != nil:
		return new(m.GetCounter().GetValue())
	case m.GetGauge() != nil:
		return new(m.GetGauge().GetValue())
	case m.GetUntyped() != nil:
		return new(m.GetUntyped().GetValue())
	}
	return nil
}

func labelOf(m *dto.Metric, name string) string {
	for _, l := range m.GetLabel() {
		if l.GetName() == name {
			return l.GetValue()
		}
	}
	return ""
}

func labelKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + "=" + labels[k] + "\x00")
	}
	return b.String()
}

func unixTime(secs float64) time.Time {
	return time.UnixMilli(int64(secs * 1000)).UTC()
}
