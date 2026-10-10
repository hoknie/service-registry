package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"svc-registry/pkg/leasejobs"
)

var KnowledgeTick = 5 * time.Second

type knowledgeSource struct{ deps Deps }

func (s knowledgeSource) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return s.deps.Knowledge.ClaimKnowledge(ctx, limit, leaseSecs)
}

func (s knowledgeSource) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return s.deps.Knowledge.ExtendKnowledge(ctx, id, leaseSecs)
}

func SpawnKnowledgeCollection(ctx context.Context, deps Deps) <-chan struct{} {
	done := make(chan struct{})
	if !deps.Config.Jobs.Enabled {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[uuid.UUID]{
		Name:   "knowledge-collect",
		Source: knowledgeSource{deps: deps},
		Run: func(ctx context.Context, id uuid.UUID) {
			if err := deps.Knowledge.RunKnowledgeCollect(ctx, id); err != nil {
				slog.Warn("documentation collection failed", "project", id, "error", err)
			}
		},
		Concurrency: int(deps.Config.Knowledge.CollectConcurrency),
		Tick:        KnowledgeTick,
	}
	go func() {
		defer close(done)
		s.Serve(ctx)
	}()
	return done
}
