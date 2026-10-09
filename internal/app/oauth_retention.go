package app

import (
	"context"
	"log/slog"
	"time"

	"svc-registry/internal/service"
)

var LoginStateRetentionEvery = time.Hour

func SpawnLoginStateRetention(ctx context.Context, state *service.State) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(LoginStateRetentionEvery)
		defer tick.Stop()
		for {
			switch n, err := service.PruneLoginStates(ctx, state); {
			case ctx.Err() != nil:
				return
			case err != nil:
				slog.Warn("login state retention failed; retrying in 1 h", "error", err)
			case n > 0:
				slog.Info("login state retention: expired states deleted", "deleted", n)
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
