package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/feature/knowledge/embeddings"
	"svc-registry/internal/feature/knowledge/search/pgvector"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/outbound"
)

type search struct {
	engine   knowledge.Engine
	external knowledge.ExternalIndex
	embedder knowledge.Embedder
}

func secretValue(raw string) (string, error) {
	if strings.HasPrefix(raw, "env:") || strings.HasPrefix(raw, "file:") {
		return config.ResolveSecretRef(raw)
	}
	return raw, nil
}

func buildSearch(all config.Config, pool *pgxpool.Pool) (search, error) {
	cfg := all.Search
	if cfg.Engine == "postgres" {
		return search{engine: knowledge.TextOnly{}}, nil
	}
	var out search
	if cfg.Embeddings() {
		key, err := secretValue(cfg.EmbeddingsAPIKey)
		if err != nil {
			return search{}, fmt.Errorf("EMBEDDINGS_API_KEY: %w", err)
		}
		out.embedder = embeddings.New(cfg.EmbeddingsURL, key, cfg.EmbeddingsModel, cfg.EmbeddingsDims, cfg.EmbeddingsBatch,
			outbound.New(all.Outbound))
	}
	switch cfg.Engine {
	case "pgvector":
		out.engine = pgvector.New(pool, cfg.EmbeddingsModel)
	default:
		if err := buildExternal(all, &out); err != nil {
			return search{}, err
		}
	}
	return out, nil
}

func (s search) check(ctx context.Context) error {
	e, ok := s.engine.(*pgvector.Engine)
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
