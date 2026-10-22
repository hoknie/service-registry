package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"svc-registry/internal/feature/knowledge/clients/embeddings"
	"svc-registry/internal/feature/knowledge/clients/search/pgvector"
	"svc-registry/internal/feature/knowledge/clients/search/qdrant"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/outbound"
)

type search struct {
	engine   knowledgeports.EnginePort
	external knowledgeports.ExternalIndexPort
	embedder knowledgeports.EmbedderPort
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
		return search{engine: knowledgetypes.TextOnly{}}, nil
	}
	var out search
	if cfg.Embeddings() {
		key, err := secretValue(cfg.EmbeddingsAPIKey)
		if err != nil {
			return search{}, fmt.Errorf("EMBEDDINGS_API_KEY: %w", err)
		}
		out.embedder = embeddings.New(cfg.EmbeddingsURL, key, cfg.EmbeddingsModel, cfg.EmbeddingsDims, cfg.EmbeddingsBatch,
			outbound.New(all.Outbound.Options()))
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
	if q, ok := s.engine.(*qdrant.EngineClient); ok {
		if err := q.Check(ctx); err != nil {
			slog.Warn("search engine check failed", "engine", q.Name(), "code", knowledgetypes.IndexFailureCode(err),
				"detail", knowledgetypes.RedactDetail(err.Error()))
		}
		return nil
	}
	e, ok := s.engine.(*pgvector.EngineClient)
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
