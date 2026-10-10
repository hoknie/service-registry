package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
	catalogservice "svc-registry/internal/feature/catalog/service"
	"svc-registry/internal/feature/forge"
	"svc-registry/internal/feature/forge/internal/repository"
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
	connections  *repository.Connections
	repositories *repository.Repos
	runs         *repository.Runs
	clients      forge.ClientFactory
	secrets      *secretbox.Box
	catalog      *catalogservice.Service
}

func New(d Deps) *Service {
	return &Service{
		cfg:          d.Config,
		connections:  repository.NewConnections(d.DB),
		repositories: repository.NewRepos(d.DB),
		runs:         repository.NewRuns(d.DB),
		clients:      d.Clients,
		secrets:      d.Secrets,
		catalog:      d.Catalog,
	}
}

func (s *Service) Managed(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	return s.repositories.Managed(ctx, ids)
}

func (s *Service) ActivitySignals(ctx context.Context, projects []uuid.UUID, _, busy bool) (map[uuid.UUID][]catalog.ProcessSignals, error) {
	return s.connections.Signals(ctx, projects, busy)
}
