package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	ingestports "svc-registry/internal/feature/ingest/ports"
	"svc-registry/internal/feature/ingest/repositories"
	"svc-registry/internal/feature/ingest/usecases"
	notifyservices "svc-registry/internal/feature/notify/services"
	topologyservices "svc-registry/internal/feature/topology/services"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/metrics"
	"svc-registry/pkg/pgtx"
)

type Deps struct {
	DB       *pgtx.DB
	Config   *config.Config
	Catalog  *catalogservices.CatalogServices
	Limiter  *auth.IngestLimiter
	Notify   *notifyservices.NotifyServices
	Topology *topologyservices.TopologyServices
	Metrics  *metrics.Metrics
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
	metrics              *metrics.Metrics
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
		metrics:              d.Metrics,
	}
}

type IngestServices struct {
	Accept  *AcceptService
	Records *RecordsService
}

func (f *IngestServices) Use(docs ingestports.DocsTriggerPort) {
	f.Accept.acceptEvent.Use(docs)
	f.Accept.recordDeployment.Use(docs)
}

func New(d Deps) *IngestServices {
	p := newParts(d)
	record := usecases.NewRecordDeployment(p.db, p.deploymentRepository, p.notify, p.metrics)
	accept := usecases.NewAcceptEvent(usecases.AcceptEventDeps{DB: p.db, Events: p.eventRepository, Deployments: p.deploymentRepository,
		RecordDeployment: record, Catalog: p.catalog, Topology: p.topology})
	f := &IngestServices{
		Accept:  &AcceptService{catalog: p.catalog, cfg: p.cfg, deploymentRepository: p.deploymentRepository, eventRepository: p.eventRepository, limiter: p.limiter, acceptEvent: accept, recordDeployment: record},
		Records: &RecordsService{catalog: p.catalog, deploymentRepository: p.deploymentRepository, eventRepository: p.eventRepository},
	}
	return f
}
