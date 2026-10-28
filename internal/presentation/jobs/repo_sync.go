package jobs

import (
	"context"
	"time"

	reposynctypes "svc-registry/internal/feature/reposync/types"
	"svc-registry/pkg/leasejobs"
)

var RepoSyncTick = 5 * time.Second

type repoSyncSource struct{ deps Deps }

func (s repoSyncSource) Claim(ctx context.Context, limit, leaseSecs int) ([]reposynctypes.Claimed, error) {
	return s.deps.RepoSync.Sync.ClaimRepoSyncRuns(ctx, limit, leaseSecs)
}

func (s repoSyncSource) Extend(ctx context.Context, c reposynctypes.Claimed, leaseSecs int) error {
	return s.deps.RepoSync.Sync.ExtendRepoSyncRun(ctx, c, leaseSecs)
}

func SpawnRepoSync(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	if !deps.Config.Jobs.Enabled {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[reposynctypes.Claimed]{
		Name:   "hosting-sync",
		Source: repoSyncSource{deps: deps},
		Run: func(ctx context.Context, c reposynctypes.Claimed) {
			_ = observe(deps, "repo_sync", func() error { return deps.RepoSync.Sync.RunClaimedSync(ctx, c) })
		},
		OnPanic:     panicked(deps, "repo_sync"),
		Concurrency: int(deps.Config.Jobs.RepoSyncConcurrency),
		Tick:        RepoSyncTick,
	}
	go func() {
		defer close(done)
		s.Serve(ctx)
	}()
	return done
}
