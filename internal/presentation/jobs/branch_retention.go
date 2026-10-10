package jobs

import (
	"context"
	"log/slog"
	"time"
)

var BranchRetentionEvery = time.Hour

func SpawnBranchRetention(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(BranchRetentionEvery)
		defer tick.Stop()
		for {
			switch n, err := deps.Catalog.PruneBranches(ctx); {
			case ctx.Err() != nil:
				return
			case err != nil:
				slog.Warn("branch retention failed; retrying at the next tick", "error", err)
			case n > 0:
				slog.Info("branch retention: expired branches deleted", "deleted", n)
			}
			if n, err := deps.Knowledge.PruneKnowledgeBlobs(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("documentation content cleanup failed; retrying at the next tick", "error", err)
			} else if n > 0 {
				slog.Info("documentation content cleanup: unreferenced content deleted", "deleted", n)
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
