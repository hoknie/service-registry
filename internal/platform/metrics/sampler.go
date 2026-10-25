package metrics

import (
	"context"
	"log/slog"
	"time"
)

type Sampler struct {
	m       *Metrics
	every   time.Duration
	sources []Source
}

func (m *Metrics) Sampler(every time.Duration, sources ...Source) *Sampler {
	return &Sampler{m: m, every: every, sources: sources}
}

func (s *Sampler) Serve(ctx context.Context) {
	t := time.NewTicker(s.every)
	defer t.Stop()
	for {
		s.SampleNow(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Sampler) SampleNow(ctx context.Context) {
	for _, src := range s.sources {
		if ctx.Err() != nil {
			return
		}
		s.sample(ctx, src)
	}
}

func (s *Sampler) sample(ctx context.Context, src Source) {
	ctx, cancel := context.WithTimeout(ctx, s.every)
	defer cancel()
	families, err := src.Collect(ctx)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		s.m.sampleErrors.WithLabelValues(src.Name).Inc()
		slog.Warn("metrics sample failed", "source", src.Name, "error", err)
		return
	}
	s.m.snapshot.put(src.Name, families, float64(time.Now().UnixMilli())/1000)
}
