package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	notifyservices "svc-registry/internal/feature/notify/services"
	reposyncports "svc-registry/internal/feature/reposync/ports"
	"svc-registry/internal/feature/reposync/repositories"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

type Deps struct {
	DB      *pgtx.DB
	Config  *config.Config
	Catalog *catalogservices.CatalogServices
	Clients reposyncports.ClientFactoryPort
	Secrets *secretbox.Box
	Notify  *notifyservices.NotifyServices
}

type parts struct {
	cfg                  *config.Config
	db                   *pgtx.DB
	connectionRepository *repositories.ConnectionRepository
	repoRepository       *repositories.RepoRepository
	exclusionRepository  *repositories.ExclusionRepository
	adoptionRepository   *repositories.AdoptionRepository
	runRepository        *repositories.RunRepository
	sources              reposyncports.ProjectSourcesPort
	clients              reposyncports.ClientFactoryPort
	secrets              *secretbox.Box
	catalog              *catalogservices.CatalogServices
	notify               *notifyservices.NotifyServices
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
		notify:               d.Notify,
	}
}

func (f *RepoSyncServices) Use(sources reposyncports.ProjectSourcesPort, docs reposyncports.DocsTriggerPort) {
	f.Unlink.sources = sources
	f.Sync.docs = docs
}

type RepoSyncServices struct {
	Connections *ConnectionsService
	Hooks       *HooksService
	Ports       *PortsService
	Previews    *PreviewsService
	Projects    *ProjectsService
	Sync        *SyncService
	Unlink      *UnlinkService
}

func New(d Deps) *RepoSyncServices {
	p := newParts(d)
	f := &RepoSyncServices{
		Connections: &ConnectionsService{catalog: p.catalog, cfg: p.cfg, connectionRepository: p.connectionRepository},
		Hooks:       &HooksService{clients: p.clients, connectionRepository: p.connectionRepository, secrets: p.secrets},
		Ports:       &PortsService{connectionRepository: p.connectionRepository, repoRepository: p.repoRepository, secrets: p.secrets},
		Previews:    &PreviewsService{catalog: p.catalog, repoRepository: p.repoRepository},
		Projects:    &ProjectsService{catalog: p.catalog, connectionRepository: p.connectionRepository, repoRepository: p.repoRepository},
		Sync:        &SyncService{adoptionRepository: p.adoptionRepository, catalog: p.catalog, cfg: p.cfg, clients: p.clients, connectionRepository: p.connectionRepository, db: p.db, exclusionRepository: p.exclusionRepository, notify: p.notify, repoRepository: p.repoRepository, runRepository: p.runRepository, secrets: p.secrets},
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
