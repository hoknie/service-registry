package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/service"
	"svc-registry/pkg/leasejobs"
)

var KnowledgeTick = 5 * time.Second

type knowledgeSource struct{ state *service.State }

func (s knowledgeSource) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return service.ClaimKnowledge(ctx, s.state, limit, leaseSecs)
}

func (s knowledgeSource) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return service.ExtendKnowledge(ctx, s.state, id, leaseSecs)
}

func SpawnKnowledgeCollection(ctx context.Context, state *service.State) <-chan struct{} {
	done := make(chan struct{})
	if !state.Config.Jobs.Enabled {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[uuid.UUID]{
		Name:   "knowledge-collect",
		Source: knowledgeSource{state: state},
		Run: func(ctx context.Context, id uuid.UUID) {
			if err := service.RunKnowledgeCollect(ctx, state, id); err != nil {
				slog.Warn("documentation collection failed", "project", id, "error", err)
			}
		},
		Concurrency: int(state.Config.Knowledge.CollectConcurrency),
		Tick:        KnowledgeTick,
	}
	go func() {
		defer close(done)
		s.Serve(ctx)
	}()
	return done
}
