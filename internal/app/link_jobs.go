package app

import (
	"context"
	"log/slog"
	"time"

	"svc-registry/internal/links"
	"svc-registry/internal/service"
	"svc-registry/pkg/leasejobs"
)

var (
	LinkCheckTick    = 5 * time.Second
	LinkRefreshEvery = time.Hour
)

type linkSource struct{ state *service.State }

func (s linkSource) Claim(ctx context.Context, limit, leaseSecs int) ([]links.Target, error) {
	return service.ClaimLinkChecks(ctx, s.state, limit, leaseSecs)
}

func (s linkSource) Extend(ctx context.Context, t links.Target, leaseSecs int) error {
	return service.ExtendLinkCheck(ctx, s.state, t, leaseSecs)
}

func SpawnLinkChecks(ctx context.Context, state *service.State) <-chan struct{} {
	done := make(chan struct{})
	if !state.Config.Jobs.Enabled {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[links.Target]{
		Name:   "link-check",
		Source: linkSource{state: state},
		Run: func(ctx context.Context, t links.Target) {
			if err := service.RunLinkCheck(ctx, state, t); err != nil {
				slog.Warn("link check not recorded", "error", err)
			}
		},
		Concurrency: int(state.Config.LinkCheck.Concurrency),
		Tick:        LinkCheckTick,
	}
	checks := make(chan struct{})
	go func() {
		defer close(checks)
		s.Serve(ctx)
	}()
	go func() {
		defer close(done)
		refreshLinks(ctx, state)
		<-checks
	}()
	return done
}

func refreshLinks(ctx context.Context, state *service.State) {
	tick := time.NewTicker(LinkRefreshEvery)
	defer tick.Stop()
	for {
		if _, err := service.RefreshLinkTargets(ctx, state); err != nil && ctx.Err() == nil {
			slog.Warn("link refresh failed; retrying at the next tick", "error", err)
		}
		switch n, err := service.PruneLinkTargets(ctx, state); {
		case ctx.Err() != nil:
			return
		case err != nil:
			slog.Warn("link target retention failed; retrying at the next tick", "error", err)
		case n > 0:
			slog.Info("link target retention: unseen addresses deleted", "deleted", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
