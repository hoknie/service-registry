package metrics

import (
	"sort"
	"sync"

	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

type snapshot struct {
	mu       sync.RWMutex
	families map[string][]*dto.MetricFamily
	stamps   map[string]float64
}

func newSnapshot() *snapshot {
	return &snapshot{families: map[string][]*dto.MetricFamily{}, stamps: map[string]float64{}}
}

func (s *snapshot) put(source string, families []*dto.MetricFamily, at float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.families[source] = families
	s.stamps[source] = at
}

func (s *snapshot) Gather() ([]*dto.MetricFamily, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sources := make([]string, 0, len(s.stamps))
	for src := range s.stamps {
		sources = append(sources, src)
	}
	sort.Strings(sources)
	var out []*dto.MetricFamily
	stamp := &dto.MetricFamily{Name: proto.String("svcr_metrics_sample_timestamp_seconds"),
		Help: proto.String("Time of the last successful sample by source, Unix seconds."), Type: dto.MetricType_GAUGE.Enum()}
	for _, src := range sources {
		out = append(out, s.families[src]...)
		stamp.Metric = append(stamp.Metric, &dto.Metric{
			Label: []*dto.LabelPair{{Name: proto.String("source"), Value: proto.String(src)}},
			Gauge: &dto.Gauge{Value: proto.Float64(s.stamps[src])},
		})
	}
	if len(stamp.Metric) > 0 {
		out = append(out, stamp)
	}
	return out, nil
}
