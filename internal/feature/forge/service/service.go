package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
	catalogservice "svc-registry/internal/feature/catalog/service"
	"svc-registry/internal/feature/forge"
	"svc-registry/internal/feature/forge/repository"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/secretbox"
)

type Deps struct {
	DB      *postgres.DB
	Config  *config.Config
	Catalog *catalogservice.Service
	Clients forge.ClientFactory
	Secrets *secretbox.Box
}

type Service struct {
	cfg          *config.Config
	db           *postgres.DB
	connections  *repository.Connections
	repositories *repository.Repos
	exclusions   *repository.Exclusions
	adoptions    *repository.Adoptions
	runs         *repository.Runs
	sources      forge.ProjectSources
	clients      forge.ClientFactory
	secrets      *secretbox.Box
	catalog      *catalogservice.Service
}

func New(d Deps) *Service {
	return &Service{
		cfg:          d.Config,
		db:           d.DB,
		connections:  repository.NewConnections(d.DB),
		repositories: repository.NewRepos(d.DB),
		exclusions:   repository.NewExclusions(d.DB),
		adoptions:    repository.NewAdoptions(d.DB),
		runs:         repository.NewRuns(d.DB),
		clients:      d.Clients,
		secrets:      d.Secrets,
		catalog:      d.Catalog,
	}
}

func (s *Service) Use(sources forge.ProjectSources) { s.sources = sources }

func (s *Service) Managed(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	return s.repositories.Managed(ctx, ids)
}

func (s *Service) ActivitySignals(ctx context.Context, projects []uuid.UUID, _, busy bool) (map[uuid.UUID][]catalog.ProcessSignals, error) {
	return s.connections.Signals(ctx, projects, busy)
}

func (s *Service) ContainerSignals(ctx context.Context, nodeID uuid.UUID) ([]catalog.ProcessSignals, error) {
	return s.connections.ContainerSignals(ctx, nodeID)
}

func (s *Service) SecretUsage(ctx context.Context, secrets []uuid.UUID) (map[uuid.UUID]int64, error) {
	return s.connections.SecretUsage(ctx, secrets)
}
