package app

import (
	"fmt"

	"svc-registry/internal/feature/knowledge/clients/search/meili"
	"svc-registry/internal/feature/knowledge/clients/search/qdrant"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/metrics"
	"svc-registry/pkg/outbound"
)

func buildExternal(all config.Config, out *search, m *metrics.Metrics) error {
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
		if m != nil {
			e.Instrument(m.GRPCInterceptor("qdrant"))
		}
		out.engine, out.external = e, e
	case "meilisearch":
		key, err := secretValue(cfg.MeilisearchAPIKey)
		if err != nil {
			return fmt.Errorf("MEILISEARCH_API_KEY: %w", err)
		}
		e := meili.New(cfg.MeilisearchURL, key, cfg.MeilisearchIndex, dims, m.Client("meilisearch", outbound.New(all.Outbound.Options())))
		out.engine, out.external = e, e
	default:
		return fmt.Errorf("KNOWLEDGE_SEARCH_ENGINE=%s is unknown", cfg.Engine)
	}
	return nil
}
