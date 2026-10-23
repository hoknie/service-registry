package jobs

import (
	"context"
	"log/slog"
	"time"
)

var OAuthRetentionEvery = time.Hour

func SpawnOAuthRetention(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(OAuthRetentionEvery)
		defer tick.Stop()
		for {
			switch n, err := deps.Access.AuthorizationServer.Prune(ctx); {
			case ctx.Err() != nil:
				return
			case err != nil:
				slog.Warn("oauth retention failed; retrying in 1 h", "error", err)
			case n > 0:
				slog.Info("oauth retention: expired requests, tokens, grants and clients deleted", "deleted", n)
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
