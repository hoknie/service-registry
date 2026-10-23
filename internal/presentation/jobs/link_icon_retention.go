package jobs

import (
	"context"
	"log/slog"
	"time"
)

var LinkIconRetentionEvery = time.Hour

func SpawnLinkIconRetention(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(LinkIconRetentionEvery)
		defer tick.Stop()
		for {
			switch n, err := deps.Links.Icons.PruneLinkIcons(ctx); {
			case ctx.Err() != nil:
				return
			case err != nil:
				slog.Warn("link icon cleanup failed; retrying at the next tick", "error", err)
			case n > 0:
				slog.Info("link icon cleanup: unreferenced icons deleted", "deleted", n)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	return done
}
