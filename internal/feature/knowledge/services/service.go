package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	forgeservices "svc-registry/internal/feature/forge/services"
	knowledgeports "svc-registry/internal/feature/knowledge/ports"
	"svc-registry/internal/feature/knowledge/repositories"
	"svc-registry/internal/feature/knowledge/usecases"
	topologyservices "svc-registry/internal/feature/topology/services"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

type Deps struct {
	DB       *pgtx.DB
	Config   *config.Config
	Catalog  *catalogservices.CatalogServices
	Forge    *forgeservices.ForgeServices
	Engine   knowledgeports.EnginePort
	External knowledgeports.ExternalIndexPort
	Embedder knowledgeports.EmbedderPort
	Readers  knowledgeports.ReaderFactoryPort
	Secrets  *secretbox.Box
	Topology *topologyservices.TopologyServices
}

type parts struct {
	cfg                  *config.Config
	db                   *pgtx.DB
	collectionRepository *repositories.CollectionRepository
	patternRepository    *repositories.PatternRepository
	snapshotRepository   *repositories.SnapshotRepository
	searchRepository     *repositories.SearchRepository
	indexRepository      *repositories.IndexRepository
	scanRepository       *repositories.ScanRepository
	sourceRepository     *repositories.SourceRepository
	sourceRepoRepository *repositories.SourceRepoRepository
	searchEngine         knowledgeports.EnginePort
	external             knowledgeports.ExternalIndexPort
	embedder             knowledgeports.EmbedderPort
	readers              knowledgeports.ReaderFactoryPort
	secrets              *secretbox.Box
	catalog              *catalogservices.CatalogServices
	forge                *forgeservices.ForgeServices
	topology             *topologyservices.TopologyServices
	saveSnapshot         *usecases.SaveSnapshotUseCase
}

func newParts(d Deps) *parts {
	snapshots := repositories.NewSnapshotRepository(d.DB)
	return &parts{
		cfg:                  d.Config,
		db:                   d.DB,
		collectionRepository: repositories.NewCollectionRepository(d.DB),
		patternRepository:    repositories.NewPatternRepository(d.DB),
		snapshotRepository:   snapshots,
		searchRepository:     repositories.NewSearchRepository(d.DB),
		indexRepository:      repositories.NewIndexRepository(d.DB),
		scanRepository:       repositories.NewScanRepository(d.DB),
		sourceRepository:     repositories.NewSourceRepository(d.DB),
		sourceRepoRepository: repositories.NewSourceRepoRepository(d.DB),
		searchEngine:         d.Engine,
		external:             d.External,
		embedder:             d.Embedder,
		readers:              d.Readers,
		secrets:              d.Secrets,
		catalog:              d.Catalog,
		forge:                d.Forge,
		topology:             d.Topology,
		saveSnapshot:         usecases.NewSaveSnapshot(d.DB, snapshots),
	}
}

type KnowledgeServices struct {
	Collect   *CollectService
	Documents *DocumentsService
	Index     *IndexService
	Ports     *PortsService
	Scans     *ScansService
	Search    *SearchService
	Settings  *SettingsService
	Sources   *SourcesService
	Space     *SpaceService
}

func New(d Deps) *KnowledgeServices {
	p := newParts(d)
	f := &KnowledgeServices{
		Collect:   &CollectService{readers: p.readers, catalog: p.catalog, cfg: p.cfg, db: p.db, forge: p.forge, scanRepository: p.scanRepository, secrets: p.secrets, collectionRepository: p.collectionRepository, snapshotRepository: p.snapshotRepository, sourceRepoRepository: p.sourceRepoRepository, sourceRepository: p.sourceRepository, topology: p.topology, saveSnapshot: p.saveSnapshot},
		Documents: &DocumentsService{catalog: p.catalog, cfg: p.cfg, collectionRepository: p.collectionRepository, snapshotRepository: p.snapshotRepository, sourceRepository: p.sourceRepository},
		Index:     &IndexService{cfg: p.cfg, db: p.db, indexRepository: p.indexRepository, searchRepository: p.searchRepository, embedder: p.embedder, external: p.external},
		Ports:     &PortsService{embedder: p.embedder, external: p.external, searchEngine: p.searchEngine, secrets: p.secrets, collectionRepository: p.collectionRepository, sourceRepository: p.sourceRepository},
		Scans:     &ScansService{catalog: p.catalog, scanRepository: p.scanRepository},
		Search:    &SearchService{collectionRepository: p.collectionRepository, indexRepository: p.indexRepository, searchRepository: p.searchRepository, embedder: p.embedder, searchEngine: p.searchEngine},
		Settings:  &SettingsService{cfg: p.cfg, db: p.db, catalog: p.catalog, collectionRepository: p.collectionRepository, patternRepository: p.patternRepository},
		Sources:   &SourcesService{catalog: p.catalog, cfg: p.cfg, db: p.db, collectionRepository: p.collectionRepository, forge: p.forge, readers: p.readers, sourceRepoRepository: p.sourceRepoRepository, sourceRepository: p.sourceRepository},
		Space:     &SpaceService{catalog: p.catalog, snapshotRepository: p.snapshotRepository},
	}
	f.Collect.documents = f.Documents
	f.Collect.settingsService = f.Settings
	f.Documents.settingsService = f.Settings
	f.Index.collect = f.Collect
	f.Index.search = f.Search
	f.Ports.sourcesService = f.Sources
	f.Ports.settingsService = f.Settings
	f.Sources.collect = f.Collect
	f.Sources.documents = f.Documents
	f.Sources.settingsService = f.Settings
	f.Space.documents = f.Documents
	return f
}
