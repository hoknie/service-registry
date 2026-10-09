package app

import (
	"context"
	"time"

	"svc-registry/internal/forge"
	"svc-registry/internal/service"
	"svc-registry/pkg/leasejobs"
)

var ForgeTick = 5 * time.Second

type forgeSource struct{ state *service.State }

func (s forgeSource) Claim(ctx context.Context, limit, leaseSecs int) ([]forge.Claimed, error) {
	return service.ClaimForgeRuns(ctx, s.state, limit, leaseSecs)
}

func (s forgeSource) Extend(ctx context.Context, c forge.Claimed, leaseSecs int) error {
	return service.ExtendForgeRun(ctx, s.state, c, leaseSecs)
}

func SpawnForgeSync(ctx context.Context, state *service.State) <-chan struct{} {
	done := make(chan struct{})
	if !state.Config.Jobs.Enabled {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[forge.Claimed]{
		Name:        "forge-sync",
		Source:      forgeSource{state: state},
		Run:         func(ctx context.Context, c forge.Claimed) { service.RunClaimedSync(ctx, state, c) },
		Concurrency: int(state.Config.Jobs.ForgeConcurrency),
		Tick:        ForgeTick,
	}
	go func() {
		defer close(done)
		s.Serve(ctx)
	}()
	return done
}
