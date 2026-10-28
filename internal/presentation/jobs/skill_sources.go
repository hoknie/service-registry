package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"svc-registry/pkg/leasejobs"
)

var (
	SkillSourcesTick        = 5 * time.Second
	SkillBlobRetentionEvery = time.Hour
)

type skillSource struct{ deps Deps }

func (s skillSource) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return s.deps.Skills.Sources.ClaimSources(ctx, limit, leaseSecs)
}

func (s skillSource) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return s.deps.Skills.Sources.ExtendSource(ctx, id, leaseSecs)
}

func SpawnSkillSources(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	if deps.Skills == nil || !deps.Config.Jobs.Enabled {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[uuid.UUID]{
		Name:   "skill-sources",
		Source: skillSource{deps: deps},
		Run: func(ctx context.Context, id uuid.UUID) {
			_ = observe(deps, "skill_sources", func() error { return deps.Skills.Sources.RunSource(ctx, id) })
		},
		OnPanic:     panicked(deps, "skill_sources"),
		Concurrency: 2,
		Tick:        SkillSourcesTick,
	}
	go func() {
		defer close(done)
		s.Serve(ctx)
	}()
	return done
}

func SpawnSkillBlobRetention(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	if deps.Skills == nil || !deps.Config.Jobs.Enabled {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		tick := time.NewTicker(SkillBlobRetentionEvery)
		defer tick.Stop()
		for {
			switch n, err := observeCount(deps, "skill_blob_retention", func() (int, error) { return deps.Skills.Ports.PruneBlobs(ctx) }); {
			case ctx.Err() != nil:
				return
			case err != nil:
				slog.Warn("skill file cleanup failed; retrying at the next tick", "error", err)
			case n > 0:
				slog.Info("skill file cleanup: unreferenced files deleted", "deleted", n)
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
