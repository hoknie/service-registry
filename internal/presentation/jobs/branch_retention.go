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
			switch n, err := observeCount(deps, "branch_retention", func() (int64, error) { return deps.Catalog.Branches.PruneBranches(ctx) }); {
			case ctx.Err() != nil:
				return
			case err != nil:
				slog.Warn("branch retention failed; retrying at the next tick", "error", err)
			case n > 0:
				slog.Info("branch retention: expired branches deleted", "deleted", n)
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
