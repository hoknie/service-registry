package service

import (
	catalogservice "svc-registry/internal/feature/catalog/service"
	"svc-registry/internal/feature/ingest/internal/repository"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
)

type Deps struct {
	DB      *postgres.DB
	Config  *config.Config
	Catalog *catalogservice.Service
	Limiter *auth.IngestLimiter
}

type Service struct {
	cfg         *config.Config
	db          *postgres.DB
	events      *repository.Events
	deployments *repository.Deployments
	limiter     *auth.IngestLimiter
	catalog     *catalogservice.Service
}

func New(d Deps) *Service {
	return &Service{
		cfg:         d.Config,
		db:          d.DB,
		events:      repository.NewEvents(d.DB),
		deployments: repository.NewDeployments(d.DB),
		limiter:     d.Limiter,
		catalog:     d.Catalog,
	}
}
