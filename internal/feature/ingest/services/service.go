package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	"svc-registry/internal/feature/ingest/repositories"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
)

type Deps struct {
	DB      *postgres.DB
	Config  *config.Config
	Catalog *catalogservices.CatalogServices
	Limiter *auth.IngestLimiter
}

type parts struct {
	cfg                  *config.Config
	db                   *postgres.DB
	eventRepository      *repositories.EventRepository
	deploymentRepository *repositories.DeploymentRepository
	limiter              *auth.IngestLimiter
	catalog              *catalogservices.CatalogServices
}

func newParts(d Deps) *parts {
	return &parts{
		cfg:                  d.Config,
		db:                   d.DB,
		eventRepository:      repositories.NewEventRepository(d.DB),
		deploymentRepository: repositories.NewDeploymentRepository(d.DB),
		limiter:              d.Limiter,
		catalog:              d.Catalog,
	}
}

type IngestServices struct {
	Accept  *AcceptService
	Records *RecordsService
}

func New(d Deps) *IngestServices {
	p := newParts(d)
	f := &IngestServices{
		Accept:  &AcceptService{catalog: p.catalog, cfg: p.cfg, db: p.db, deploymentRepository: p.deploymentRepository, eventRepository: p.eventRepository, limiter: p.limiter},
		Records: &RecordsService{catalog: p.catalog, deploymentRepository: p.deploymentRepository, eventRepository: p.eventRepository},
	}
	return f
}
