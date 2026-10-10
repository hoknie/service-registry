package app

import (
	"fmt"

	"svc-registry/internal/feature/knowledge/search/meili"
	"svc-registry/internal/feature/knowledge/search/qdrant"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/outbound"
)

func buildExternal(all config.Config, out *search) error {
	cfg := all.Search
	dims := 0
	if out.embedder != nil {
		dims = cfg.EmbeddingsDims
	}
	switch cfg.Engine {
	case "qdrant":
		key, err := secretValue(cfg.QdrantAPIKey)
		if err != nil {
			return fmt.Errorf("QDRANT_API_KEY: %w", err)
		}
		e := qdrant.New(cfg.QdrantURL, key, cfg.QdrantCollection, dims)
		out.engine, out.external = e, e
	case "meilisearch":
		key, err := secretValue(cfg.MeilisearchAPIKey)
		if err != nil {
			return fmt.Errorf("MEILISEARCH_API_KEY: %w", err)
		}
		e := meili.New(cfg.MeilisearchURL, key, cfg.MeilisearchIndex, dims, outbound.New(all.Outbound))
		out.engine, out.external = e, e
	default:
		return fmt.Errorf("KNOWLEDGE_SEARCH_ENGINE=%s is unknown", cfg.Engine)
	}
	return nil
}
