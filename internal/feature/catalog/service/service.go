package service

import (
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/catalog/repository"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/secretbox"
)

type Deps struct {
	DB      *postgres.DB
	Config  *config.Config
	Secrets *secretbox.Box
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
	secrets     *repository.Secrets
	box         *secretbox.Box
	secretUsers []catalog.SecretUser
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
		secrets:     repository.NewSecrets(d.DB),
		box:         d.Secrets,
	}
}

func (s *Service) Use(managed catalog.ManagedNodes, sources ...catalog.ActivitySource) {
	s.managed = managed
	s.sources = sources
}

func (s *Service) UseSecretUsers(users ...catalog.SecretUser) { s.secretUsers = users }

func (s *Service) Wired() bool {
	return s.managed != nil && len(s.sources) > 0 && len(s.secretUsers) > 0
}
