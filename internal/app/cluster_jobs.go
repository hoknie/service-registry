package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/service"
	"svc-registry/pkg/leasejobs"
)

var (
	ClusterTick            = 5 * time.Second
	WorkloadRetentionEvery = time.Hour
)

type clusterSource struct{ state *service.State }

func (s clusterSource) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return service.ClaimClusterPolls(ctx, s.state, limit, leaseSecs)
}

func (s clusterSource) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return service.ExtendClusterPoll(ctx, s.state, id, leaseSecs)
}

func SpawnClusterPolls(ctx context.Context, state *service.State) <-chan struct{} {
	done := make(chan struct{})
	if !state.Config.Jobs.Enabled {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[uuid.UUID]{
		Name:   "cluster-poll",
		Source: clusterSource{state: state},
		Run: func(ctx context.Context, id uuid.UUID) {
			if err := service.RunClusterPoll(ctx, state, id); err != nil {
				slog.Warn("cluster poll failed", "cluster", id, "error", err)
			}
		},
		Concurrency: int(state.Config.K8s.PollConcurrency),
		Tick:        ClusterTick,
	}
	polls := make(chan struct{})
	go func() {
		defer close(polls)
		s.Serve(ctx)
	}()
	go func() {
		defer close(done)
		pruneWorkloads(ctx, state)
		<-polls
	}()
	return done
}

func pruneWorkloads(ctx context.Context, state *service.State) {
	tick := time.NewTicker(WorkloadRetentionEvery)
	defer tick.Stop()
	for {
		switch n, err := service.PruneWorkloads(ctx, state); {
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
