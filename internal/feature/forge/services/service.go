package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	forgeports "svc-registry/internal/feature/forge/ports"
	"svc-registry/internal/feature/forge/repositories"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

type Deps struct {
	DB      *pgtx.DB
	Config  *config.Config
	Catalog *catalogservices.CatalogServices
	Clients forgeports.ClientFactoryPort
	Secrets *secretbox.Box
}

type parts struct {
	cfg                  *config.Config
	db                   *pgtx.DB
	connectionRepository *repositories.ConnectionRepository
	repoRepository       *repositories.RepoRepository
	exclusionRepository  *repositories.ExclusionRepository
	adoptionRepository   *repositories.AdoptionRepository
	runRepository        *repositories.RunRepository
	sources              forgeports.ProjectSourcesPort
	clients              forgeports.ClientFactoryPort
	secrets              *secretbox.Box
	catalog              *catalogservices.CatalogServices
}

func newParts(d Deps) *parts {
	return &parts{
		cfg:                  d.Config,
		db:                   d.DB,
		connectionRepository: repositories.NewConnectionRepository(d.DB),
		repoRepository:       repositories.NewRepoRepository(d.DB),
		exclusionRepository:  repositories.NewExclusionRepository(d.DB),
		adoptionRepository:   repositories.NewAdoptionRepository(d.DB),
		runRepository:        repositories.NewRunRepository(d.DB),
		clients:              d.Clients,
		secrets:              d.Secrets,
		catalog:              d.Catalog,
	}
}

func (f *ForgeServices) Use(sources forgeports.ProjectSourcesPort) { f.Unlink.sources = sources }

type ForgeServices struct {
	Connections *ConnectionsService
	Hooks       *HooksService
	Ports       *PortsService
	Previews    *PreviewsService
	Projects    *ProjectsService
	Sync        *SyncService
	Unlink      *UnlinkService
}

func New(d Deps) *ForgeServices {
	p := newParts(d)
	f := &ForgeServices{
		Connections: &ConnectionsService{catalog: p.catalog, cfg: p.cfg, connectionRepository: p.connectionRepository},
		Hooks:       &HooksService{clients: p.clients, connectionRepository: p.connectionRepository, secrets: p.secrets},
		Ports:       &PortsService{connectionRepository: p.connectionRepository, repoRepository: p.repoRepository, secrets: p.secrets},
		Previews:    &PreviewsService{catalog: p.catalog, repoRepository: p.repoRepository},
		Projects:    &ProjectsService{catalog: p.catalog, connectionRepository: p.connectionRepository, repoRepository: p.repoRepository},
		Sync:        &SyncService{adoptionRepository: p.adoptionRepository, catalog: p.catalog, cfg: p.cfg, clients: p.clients, connectionRepository: p.connectionRepository, exclusionRepository: p.exclusionRepository, repoRepository: p.repoRepository, runRepository: p.runRepository, secrets: p.secrets},
		Unlink:      &UnlinkService{adoptionRepository: p.adoptionRepository, catalog: p.catalog, connectionRepository: p.connectionRepository, db: p.db, exclusionRepository: p.exclusionRepository, repoRepository: p.repoRepository, sources: p.sources},
	}
	f.Connections.sync = f.Sync
	f.Hooks.connectionsService = f.Connections
	f.Hooks.sync = f.Sync
	f.Previews.connectionsService = f.Connections
	f.Previews.sync = f.Sync
	f.Projects.sync = f.Sync
	f.Sync.connectionsService = f.Connections
	f.Unlink.connectionsService = f.Connections
	f.Sync.unlink = f.Unlink
	return f
}
