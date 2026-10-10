package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/feature/knowledge"
	"svc-registry/pkg/leasejobs"
)

var IndexTick = 5 * time.Second

type indexSource struct{ deps Deps }

func (s indexSource) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return s.deps.Knowledge.ClaimIndex(ctx, limit, leaseSecs)
}

func (s indexSource) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return s.deps.Knowledge.ExtendIndex(ctx, id, leaseSecs)
}

func SpawnKnowledgeIndex(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	info := deps.Knowledge.Indexing()
	e := info.Engine
	indexing := deps.Config.Jobs.Enabled && info.Enabled
	slog.Info("documentation search", "engine", e.Name(), "modes", e.Modes(), "default", e.DefaultMode(),
		"embeddings_model", info.Model, "indexing", indexing)
	if !indexing {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[uuid.UUID]{
		Name:   "knowledge-index",
		Source: indexSource{deps: deps},
		Run: func(ctx context.Context, id uuid.UUID) {
			err := deps.Knowledge.RunKnowledgeIndex(ctx, id)
			var failure *knowledge.IndexFailure
			switch {
			case errors.As(err, &failure):
				slog.Warn("documentation indexing failed", "project", id, "code", failure.Code, "detail", failure.Detail)
			case err != nil:
				slog.Warn("documentation indexing failed", "project", id, "error", err)
			}
		},
		Concurrency: 1,
		Tick:        IndexTick,
	}
	go func() {
		defer close(done)
		go retainLoop(ctx, deps)
		s.Serve(ctx)
	}()
	return done
}

var RetainEvery = time.Hour

func retainLoop(ctx context.Context, deps Deps) {
	if !deps.Knowledge.Indexing().External {
		return
	}
	t := time.NewTicker(RetainEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := deps.Knowledge.RetainExternalIndex(ctx); err != nil {
				slog.Warn("search index cleanup failed", "error", err)
			}
		}
	}
}
