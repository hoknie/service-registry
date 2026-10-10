package jobs

import (
	"context"
	"log/slog"
	"time"

	"svc-registry/internal/platform/config"
)

var BootstrapRetry = 5 * time.Second

func SpawnBootstrapAdmin(ctx context.Context, deps Deps, admin config.BootstrapAdmin) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		row, err := deps.Access.PrepareBootstrapAdmin(admin)
		if err != nil {
			slog.Error("bootstrap admin: cannot prepare the account", "error", err)
			return
		}
		warned := false
		for {
			created, err := deps.Access.EnsureBootstrapAdmin(ctx, row)
			switch {
			case err == nil && created:
				slog.Info("bootstrap admin created", "email", row.Email)
				return
			case err == nil:
				slog.Info("bootstrap admin skipped: users already exist")
				return
			case !warned:
				slog.Warn("bootstrap admin: database not ready, retrying every 5 s", "error", err)
				warned = true
			default:
				slog.Debug("bootstrap admin: retrying", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(BootstrapRetry):
			}
		}
	}()
	return done
}
