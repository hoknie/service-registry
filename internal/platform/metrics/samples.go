package metrics

import (
	"context"
	"sort"

	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

type Sample struct {
	Name    string
	Labels  map[string]string
	Value   float64
	Counter bool
}

type Source struct {
	Name    string
	Collect func(ctx context.Context) ([]*dto.MetricFamily, error)
}

func SampleSource(name string, fns ...func(context.Context) ([]Sample, error)) Source {
	return Source{Name: name, Collect: func(ctx context.Context) ([]*dto.MetricFamily, error) {
		var all []Sample
		for _, fn := range fns {
			s, err := fn(ctx)
			if err != nil {
				return nil, err
			}
			all = append(all, s...)
		}
		return Families(all), nil
	}}
}

func Families(samples []Sample) []*dto.MetricFamily {
	byName := map[string]*dto.MetricFamily{}
	var names []string
	for _, s := range samples {
		f, ok := byName[s.Name]
		if !ok {
			kind := dto.MetricType_GAUGE
			if s.Counter {
				kind = dto.MetricType_COUNTER
			}
			f = &dto.MetricFamily{Name: proto.String(s.Name), Help: proto.String(helpOf(s.Name)), Type: kind.Enum()}
			byName[s.Name] = f
			names = append(names, s.Name)
		}
		m := &dto.Metric{Label: labelPairs(s.Labels)}
		if f.GetType() == dto.MetricType_COUNTER {
			m.Counter = &dto.Counter{Value: proto.Float64(s.Value)}
		} else {
			m.Gauge = &dto.Gauge{Value: proto.Float64(s.Value)}
		}
		f.Metric = append(f.Metric, m)
	}
	sort.Strings(names)
	out := make([]*dto.MetricFamily, 0, len(names))
	for _, n := range names {
		out = append(out, byName[n])
	}
	return out
}

func labelPairs(labels map[string]string) []*dto.LabelPair {
	pairs := make([]*dto.LabelPair, 0, len(labels))
	for k, v := range labels {
		pairs = append(pairs, &dto.LabelPair{Name: proto.String(k), Value: proto.String(v)})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].GetName() < pairs[j].GetName() })
	return pairs
}
