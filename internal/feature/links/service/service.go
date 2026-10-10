package service

import (
	catalogservice "svc-registry/internal/feature/catalog/service"
	"svc-registry/internal/feature/links"
	"svc-registry/internal/feature/links/internal/repository"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
)

type Deps struct {
	DB      *postgres.DB
	Config  *config.Config
	Catalog *catalogservice.Service
	Checker links.Checker
	Icons   links.IconFiles
}

type Service struct {
	cfg       *config.Config
	kinds     *repository.Kinds
	templates *repository.Templates
	targets   *repository.Targets
	checker   links.Checker
	icons     links.IconFiles
	catalog   *catalogservice.Service
}

func New(d Deps) *Service {
	return &Service{
		cfg:       d.Config,
		kinds:     repository.NewKinds(d.DB),
		templates: repository.NewTemplates(d.DB),
		targets:   repository.NewTargets(d.DB),
		checker:   d.Checker,
		icons:     d.Icons,
		catalog:   d.Catalog,
	}
}
