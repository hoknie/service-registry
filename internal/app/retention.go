package app

import (
	"context"
	"log/slog"
	"time"

	"svc-registry/internal/service"
)

var RetentionEvery = time.Hour

func SpawnEventRetention(ctx context.Context, state *service.State) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(RetentionEvery)
		defer tick.Stop()
		for {
			switch n, err := service.PruneEvents(ctx, state); {
			case ctx.Err() != nil:
				return
			case err != nil:
				slog.Warn("event retention failed; retrying in 1 h", "error", err)
			case n == 0:
				slog.Debug("event retention: nothing to delete")
			default:
				slog.Info("event retention: old events deleted", "deleted", n)
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
