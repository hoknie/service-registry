package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"svc-registry/pkg/leasejobs"
)

var (
	ClusterTick            = 5 * time.Second
	WorkloadRetentionEvery = time.Hour
)

type clusterSource struct{ deps Deps }

func (s clusterSource) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return s.deps.Deploy.ClaimClusterPolls(ctx, limit, leaseSecs)
}

func (s clusterSource) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return s.deps.Deploy.ExtendClusterPoll(ctx, id, leaseSecs)
}

func SpawnClusterPolls(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	if !deps.Config.Jobs.Enabled {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[uuid.UUID]{
		Name:   "cluster-poll",
		Source: clusterSource{deps: deps},
		Run: func(ctx context.Context, id uuid.UUID) {
			if err := deps.Deploy.RunClusterPoll(ctx, id); err != nil {
				slog.Warn("cluster poll failed", "cluster", id, "error", err)
			}
		},
		Concurrency: int(deps.Config.K8s.PollConcurrency),
		Tick:        ClusterTick,
	}
	polls := make(chan struct{})
	go func() {
		defer close(polls)
		s.Serve(ctx)
	}()
	go func() {
		defer close(done)
		pruneWorkloads(ctx, deps)
		<-polls
	}()
	return done
}

func pruneWorkloads(ctx context.Context, deps Deps) {
	tick := time.NewTicker(WorkloadRetentionEvery)
	defer tick.Stop()
	for {
		switch n, err := deps.Deploy.PruneWorkloads(ctx); {
		case ctx.Err() != nil:
			return
		case err != nil:
			slog.Warn("workload retention failed; retrying at the next tick", "error", err)
		case n > 0:
			slog.Info("workload retention: vanished observations deleted", "deleted", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
