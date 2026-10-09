package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/config"
	"svc-registry/internal/embeddings"
	"svc-registry/internal/knowledge"
	"svc-registry/internal/outbound"
	"svc-registry/internal/search/pgvector"
	"svc-registry/internal/service"
	"svc-registry/pkg/leasejobs"
)

var IndexTick = 5 * time.Second

func secretValue(raw string) (string, error) {
	if strings.HasPrefix(raw, "env:") || strings.HasPrefix(raw, "file:") {
		return config.ResolveSecretRef(raw)
	}
	return raw, nil
}

func buildSearch(state *service.State) error {
	cfg := state.Config.Search
	if cfg.Engine == "postgres" {
		state.SearchEngine = knowledge.TextOnly{}
		return nil
	}
	if cfg.Embeddings() {
		key, err := secretValue(cfg.EmbeddingsAPIKey)
		if err != nil {
			return fmt.Errorf("EMBEDDINGS_API_KEY: %w", err)
		}
		state.Embedder = embeddings.New(cfg.EmbeddingsURL, key, cfg.EmbeddingsModel, cfg.EmbeddingsDims, cfg.EmbeddingsBatch, outbound.New(state.Config.Outbound))
	}
	switch cfg.Engine {
	case "pgvector":
		state.SearchEngine = pgvector.New(state.DB, cfg.EmbeddingsModel)
	default:
		return buildExternal(state)
	}
	return nil
}

func CheckSearch(ctx context.Context, state *service.State) error {
	e, ok := state.SearchEngine.(*pgvector.Engine)
	if !ok {
		return nil
	}
	err := e.Check(ctx)
	if err == pgvector.ErrNoExtension {
		return err
	}
	if err != nil {
		slog.Warn("search engine check skipped", "error", err)
	}
	return nil
}

type indexSource struct{ state *service.State }

func (s indexSource) Claim(ctx context.Context, limit, leaseSecs int) ([]uuid.UUID, error) {
	return service.ClaimIndex(ctx, s.state, limit, leaseSecs)
}

func (s indexSource) Extend(ctx context.Context, id uuid.UUID, leaseSecs int) error {
	return service.ExtendIndex(ctx, s.state, id, leaseSecs)
}

func SpawnKnowledgeIndex(ctx context.Context, state *service.State) <-chan struct{} {
	done := make(chan struct{})
	e := state.SearchEngine
	model := ""
	if state.Embedder != nil {
		model = state.Embedder.Model()
	}
	indexing := state.Config.Jobs.Enabled && (state.Embedder != nil || state.ExternalIndex != nil)
	slog.Info("documentation search", "engine", e.Name(), "modes", e.Modes(), "default", e.DefaultMode(),
		"embeddings_model", model, "indexing", indexing)
	if !indexing {
		close(done)
		return done
	}
	s := &leasejobs.Scheduler[uuid.UUID]{
		Name:   "knowledge-index",
		Source: indexSource{state: state},
		Run: func(ctx context.Context, id uuid.UUID) {
			if err := service.RunKnowledgeIndex(ctx, state, id); err != nil {
				slog.Warn("documentation indexing failed", "project", id, "error", err)
			}
		},
		Concurrency: 1,
		Tick:        IndexTick,
	}
	go func() {
		defer close(done)
		go retainLoop(ctx, state)
		s.Serve(ctx)
	}()
	return done
}

var RetainEvery = time.Hour

func retainLoop(ctx context.Context, state *service.State) {
	if state.ExternalIndex == nil {
		return
	}
	t := time.NewTicker(RetainEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := service.RetainExternalIndex(ctx, state); err != nil {
				slog.Warn("search index cleanup failed", "error", err)
			}
		}
	}
}
