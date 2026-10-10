package jobs

import (
	"context"
	"time"

	"svc-registry/internal/feature/forge"
	"svc-registry/pkg/leasejobs"
)

var ForgeTick = 5 * time.Second

type forgeSource struct{ deps Deps }

func (s forgeSource) Claim(ctx context.Context, limit, leaseSecs int) ([]forge.Claimed, error) {
	return s.deps.Forge.ClaimForgeRuns(ctx, limit, leaseSecs)
}

func (s forgeSource) Extend(ctx context.Context, c forge.Claimed, leaseSecs int) error {
	return s.deps.Forge.ExtendForgeRun(ctx, c, leaseSecs)
}

func SpawnForgeSync(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	if !deps.Config.Jobs.Enabled {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[forge.Claimed]{
		Name:        "forge-sync",
		Source:      forgeSource{deps: deps},
		Run:         func(ctx context.Context, c forge.Claimed) { deps.Forge.RunClaimedSync(ctx, c) },
		Concurrency: int(deps.Config.Jobs.ForgeConcurrency),
		Tick:        ForgeTick,
	}
	go func() {
		defer close(done)
		s.Serve(ctx)
	}()
	return done
}
