package metrics

import (
	"sort"
	"strings"
)

type Shape struct {
	Name   string
	Labels []string
}

func FromPairs(pairs map[string]int64, shapes map[string]Shape) []Sample {
	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Sample, 0, len(keys))
	for _, k := range keys {
		prefix, rest, _ := strings.Cut(k, ":")
		shape, ok := shapes[prefix]
		if !ok {
			continue
		}
		values := []string{}
		if rest != "" {
			values = strings.Split(rest, "/")
		}
		if len(values) != len(shape.Labels) {
			continue
		}
		labels := make(map[string]string, len(values))
		for i, name := range shape.Labels {
			labels[name] = values[i]
		}
		out = append(out, Sample{Name: shape.Name, Labels: labels, Value: float64(pairs[k])})
	}
	return out
}

func Zero(pairs map[string]int64, keys ...string) map[string]int64 {
	for _, k := range keys {
		if _, ok := pairs[k]; !ok {
			pairs[k] = 0
		}
	}
	return pairs
}
