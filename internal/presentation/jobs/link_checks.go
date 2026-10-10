package jobs

import (
	"context"
	"log/slog"
	"time"

	"svc-registry/internal/feature/links"
	"svc-registry/pkg/leasejobs"
)

var (
	LinkCheckTick    = 5 * time.Second
	LinkRefreshEvery = time.Hour
)

type linkSource struct{ deps Deps }

func (s linkSource) Claim(ctx context.Context, limit, leaseSecs int) ([]links.Target, error) {
	return s.deps.Links.ClaimLinkChecks(ctx, limit, leaseSecs)
}

func (s linkSource) Extend(ctx context.Context, t links.Target, leaseSecs int) error {
	return s.deps.Links.ExtendLinkCheck(ctx, t, leaseSecs)
}

func SpawnLinkChecks(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	if !deps.Config.Jobs.Enabled {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[links.Target]{
		Name:   "link-check",
		Source: linkSource{deps: deps},
		Run: func(ctx context.Context, t links.Target) {
			if err := deps.Links.RunLinkCheck(ctx, t); err != nil {
				slog.Warn("link check not recorded", "error", err)
			}
		},
		Concurrency: int(deps.Config.LinkCheck.Concurrency),
		Tick:        LinkCheckTick,
	}
	checks := make(chan struct{})
	go func() {
		defer close(checks)
		s.Serve(ctx)
	}()
	go func() {
		defer close(done)
		refreshLinks(ctx, deps)
		<-checks
	}()
	return done
}

func refreshLinks(ctx context.Context, deps Deps) {
	tick := time.NewTicker(LinkRefreshEvery)
	defer tick.Stop()
	for {
		if _, err := deps.Links.RefreshLinkTargets(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("link refresh failed; retrying at the next tick", "error", err)
		}
		switch n, err := deps.Links.PruneLinkTargets(ctx); {
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
