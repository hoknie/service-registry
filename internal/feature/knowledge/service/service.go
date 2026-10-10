package service

import (
	catalogservice "svc-registry/internal/feature/catalog/service"
	forgeservice "svc-registry/internal/feature/forge/service"
	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/feature/knowledge/internal/repository"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/secretbox"
)

type Deps struct {
	DB       *postgres.DB
	Config   *config.Config
	Catalog  *catalogservice.Service
	Forge    *forgeservice.Service
	Engine   knowledge.Engine
	External knowledge.ExternalIndex
	Embedder knowledge.Embedder
	Readers  knowledge.ReaderFactory
	Secrets  *secretbox.Box
}

type Service struct {
	cfg          *config.Config
	db           *postgres.DB
	settings     *repository.Collections
	patterns     *repository.Patterns
	snapshots    *repository.Snapshots
	docSearch    *repository.Searches
	docIndex     *repository.Indexes
	scans        *repository.Scans
	sources      *repository.Sources
	searchEngine knowledge.Engine
	external     knowledge.ExternalIndex
	embedder     knowledge.Embedder
	readers      knowledge.ReaderFactory
	secrets      *secretbox.Box
	catalog      *catalogservice.Service
	forge        *forgeservice.Service
}

func New(d Deps) *Service {
	return &Service{
		cfg:          d.Config,
		db:           d.DB,
		settings:     repository.NewCollections(d.DB),
		patterns:     repository.NewPatterns(d.DB),
		snapshots:    repository.NewSnapshots(d.DB),
		docSearch:    repository.NewSearches(d.DB),
		docIndex:     repository.NewIndexes(d.DB),
		scans:        repository.NewScans(d.DB),
		sources:      repository.NewSources(d.DB),
		searchEngine: d.Engine,
		external:     d.External,
		embedder:     d.Embedder,
		readers:      d.Readers,
		secrets:      d.Secrets,
		catalog:      d.Catalog,
		forge:        d.Forge,
	}
}
