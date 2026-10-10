package service

import (
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/catalog/internal/repository"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
)

type Deps struct {
	DB     *postgres.DB
	Config *config.Config
}

type Service struct {
	cfg         *config.Config
	db          *postgres.DB
	nodes       *repository.Nodes
	bindings    *repository.Bindings
	projectKeys *repository.ProjectKeys
	branches    *repository.Branches
	activity    *repository.Activities
	managed     catalog.ManagedNodes
	sources     []catalog.ActivitySource
}

func New(d Deps) *Service {
	return &Service{
		cfg:         d.Config,
		db:          d.DB,
		nodes:       repository.NewNodes(d.DB),
		bindings:    repository.NewBindings(d.DB),
		projectKeys: repository.NewProjectKeys(d.DB),
		branches:    repository.NewBranches(d.DB, d.Config.Branches.StaleDays),
		activity:    repository.NewActivities(d.DB),
	}
}

func (s *Service) Use(managed catalog.ManagedNodes, sources ...catalog.ActivitySource) {
	s.managed = managed
	s.sources = sources
}

func (s *Service) Wired() bool { return s.managed != nil && len(s.sources) > 0 }
