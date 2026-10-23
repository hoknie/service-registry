package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	ingestports "svc-registry/internal/feature/ingest/ports"
	"svc-registry/internal/feature/ingest/repositories"
	notifyservices "svc-registry/internal/feature/notify/services"
	topologyservices "svc-registry/internal/feature/topology/services"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

type Deps struct {
	DB       *pgtx.DB
	Config   *config.Config
	Catalog  *catalogservices.CatalogServices
	Limiter  *auth.IngestLimiter
	Notify   *notifyservices.NotifyServices
	Topology *topologyservices.TopologyServices
}

type parts struct {
	cfg                  *config.Config
	db                   *pgtx.DB
	eventRepository      *repositories.EventRepository
	deploymentRepository *repositories.DeploymentRepository
	limiter              *auth.IngestLimiter
	catalog              *catalogservices.CatalogServices
	notify               *notifyservices.NotifyServices
	topology             *topologyservices.TopologyServices
}

func newParts(d Deps) *parts {
	return &parts{
		cfg:                  d.Config,
		db:                   d.DB,
		eventRepository:      repositories.NewEventRepository(d.DB),
		deploymentRepository: repositories.NewDeploymentRepository(d.DB),
		limiter:              d.Limiter,
		catalog:              d.Catalog,
		notify:               d.Notify,
		topology:             d.Topology,
	}
}

type IngestServices struct {
	Accept  *AcceptService
	Records *RecordsService
}

func (f *IngestServices) Use(docs ingestports.DocsTriggerPort) { f.Accept.docs = docs }

func New(d Deps) *IngestServices {
	p := newParts(d)
	f := &IngestServices{
		Accept:  &AcceptService{catalog: p.catalog, cfg: p.cfg, db: p.db, deploymentRepository: p.deploymentRepository, eventRepository: p.eventRepository, limiter: p.limiter, notify: p.notify, topology: p.topology},
		Records: &RecordsService{catalog: p.catalog, deploymentRepository: p.deploymentRepository, eventRepository: p.eventRepository},
	}
	return f
}
