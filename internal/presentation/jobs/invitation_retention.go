package jobs

import (
	"context"
	"log/slog"
	"time"
)

var InvitationRetentionEvery = time.Hour

func SpawnInvitationRetention(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	if deps.Membership == nil {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		tick := time.NewTicker(InvitationRetentionEvery)
		defer tick.Stop()
		for {
			switch n, err := observeCount(deps, "invitation_retention", func() (int64, error) { return deps.Membership.Invitations.PruneInvitations(ctx) }); {
			case ctx.Err() != nil:
				return
			case err != nil:
				slog.Warn("invitation retention failed; retrying in 1 h", "error", err)
			case n > 0:
				slog.Info("invitation retention: closed invitations deleted", "deleted", n)
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
