package jobs

import (
	"context"
	"log/slog"
	"time"
)

var KnowledgeBlobRetentionEvery = time.Hour

func SpawnKnowledgeBlobRetention(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(KnowledgeBlobRetentionEvery)
		defer tick.Stop()
		for {
			switch n, err := observeCount(deps, "knowledge_blob_retention", func() (int64, error) { return deps.Knowledge.Collect.PruneKnowledgeBlobs(ctx) }); {
			case ctx.Err() != nil:
				return
			case err != nil:
				slog.Warn("documentation content cleanup failed; retrying at the next tick", "error", err)
			case n > 0:
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
