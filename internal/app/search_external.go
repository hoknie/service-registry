package app

import (
	"fmt"

	"svc-registry/internal/outbound"
	"svc-registry/internal/search/meili"
	"svc-registry/internal/search/qdrant"
	"svc-registry/internal/service"
)

func buildExternal(state *service.State) error {
	cfg := state.Config.Search
	dims := 0
	if state.Embedder != nil {
		dims = cfg.EmbeddingsDims
	}
	switch cfg.Engine {
	case "qdrant":
		key, err := secretValue(cfg.QdrantAPIKey)
		if err != nil {
			return fmt.Errorf("QDRANT_API_KEY: %w", err)
		}
		e := qdrant.New(cfg.QdrantURL, key, cfg.QdrantCollection, dims)
		state.SearchEngine, state.ExternalIndex = e, e
	case "meilisearch":
		key, err := secretValue(cfg.MeilisearchAPIKey)
		if err != nil {
			return fmt.Errorf("MEILISEARCH_API_KEY: %w", err)
		}
		e := meili.New(cfg.MeilisearchURL, key, cfg.MeilisearchIndex, dims, outbound.New(state.Config.Outbound))
		state.SearchEngine, state.ExternalIndex = e, e
	default:
		return fmt.Errorf("KNOWLEDGE_SEARCH_ENGINE=%s is unknown", cfg.Engine)
	}
	return nil
}
