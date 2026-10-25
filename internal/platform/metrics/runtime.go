package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	dto "github.com/prometheus/client_model/go"
)

func RuntimeSource() Source {
	r := prometheus.NewRegistry()
	r.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return Source{Name: "runtime", Collect: func(context.Context) ([]*dto.MetricFamily, error) { return r.Gather() }}
}

type PoolStat struct {
	Acquired, Idle, Constructing, Max int32
	Acquires, EmptyAcquires           int64
	AcquireWait                       time.Duration
}

func PoolSource(stat func() PoolStat) Source {
	return SampleSource("pool", func(context.Context) ([]Sample, error) {
		s := stat()
		state := func(v int32, name string) Sample {
			return Sample{Name: "svcr_db_pool_connections", Labels: map[string]string{"state": name}, Value: float64(v)}
		}
		return []Sample{
			state(s.Acquired, "acquired"), state(s.Idle, "idle"), state(s.Constructing, "constructing"),
			{Name: "svcr_db_pool_max_connections", Value: float64(s.Max)},
			{Name: "svcr_db_pool_acquires_total", Value: float64(s.Acquires), Counter: true},
			{Name: "svcr_db_pool_acquire_wait_seconds_total", Value: s.AcquireWait.Seconds(), Counter: true},
			{Name: "svcr_db_pool_empty_acquires_total", Value: float64(s.EmptyAcquires), Counter: true},
		}, nil
	})
}
