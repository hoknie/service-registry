package config

import (
	"net"
	"net/url"
	"strings"
)

type SearchConfig struct {
	Engine            string `env:"KNOWLEDGE_SEARCH_ENGINE" envDefault:"postgres" validate:"oneof=postgres pgvector qdrant meilisearch"`
	EmbeddingsURL     string `env:"EMBEDDINGS_URL"`
	EmbeddingsModel   string `env:"EMBEDDINGS_MODEL"`
	EmbeddingsAPIKey  string `env:"EMBEDDINGS_API_KEY"`
	EmbeddingsDims    int    `env:"EMBEDDINGS_DIMENSIONS" validate:"omitempty,min=8,max=8192"`
	EmbeddingsBatch   int    `env:"EMBEDDINGS_BATCH" envDefault:"32" validate:"min=1,max=512"`
	ChunkChars        int    `env:"KNOWLEDGE_CHUNK_CHARS" envDefault:"1500" validate:"min=200,max=8000"`
	IndexIntervalSecs uint32 `env:"KNOWLEDGE_INDEX_INTERVAL_SECS" envDefault:"60" validate:"min=5,max=86400"`
	QdrantURL         string `env:"QDRANT_URL"`
	QdrantAPIKey      string `env:"QDRANT_API_KEY"`
	QdrantCollection  string `env:"QDRANT_COLLECTION" envDefault:"svc_registry_docs"`
	MeilisearchURL    string `env:"MEILISEARCH_URL"`
	MeilisearchAPIKey string `env:"MEILISEARCH_API_KEY"`
	MeilisearchIndex  string `env:"MEILISEARCH_INDEX" envDefault:"svc_registry_docs"`
}

func (c *SearchConfig) Embeddings() bool { return c.EmbeddingsURL != "" }

func httpURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func (c *SearchConfig) check() Errors {
	var errs Errors
	if c.EmbeddingsURL != "" {
		if !httpURL(c.EmbeddingsURL) {
			errs = append(errs, &Error{Var: "EMBEDDINGS_URL", Reason: "must be an http(s) URL"})
		}
		if strings.TrimSpace(c.EmbeddingsModel) == "" {
			errs = append(errs, &Error{Var: "EMBEDDINGS_MODEL", Reason: "is required with EMBEDDINGS_URL"})
		}
		if c.EmbeddingsDims == 0 {
			errs = append(errs, &Error{Var: "EMBEDDINGS_DIMENSIONS", Reason: "is required with EMBEDDINGS_URL"})
		}
	}
	if (c.Engine == "pgvector" || c.Engine == "qdrant") && c.EmbeddingsURL == "" {
		errs = append(errs, &Error{Var: "EMBEDDINGS_URL", Reason: "is required by KNOWLEDGE_SEARCH_ENGINE=" + c.Engine})
	}
	if c.Engine == "qdrant" {
		if _, _, err := net.SplitHostPort(c.QdrantURL); c.QdrantURL == "" || err != nil {
			errs = append(errs, &Error{Var: "QDRANT_URL", Reason: "must be host:port of the Qdrant gRPC API"})
		}
	}
	if c.Engine == "meilisearch" && !httpURL(c.MeilisearchURL) {
		errs = append(errs, &Error{Var: "MEILISEARCH_URL", Reason: "must be an http(s) URL"})
	}
	return errs
}
