package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
	catalogservice "svc-registry/internal/feature/catalog/service"
	"svc-registry/internal/feature/deploy"
	"svc-registry/internal/feature/deploy/repository"
	ingestservice "svc-registry/internal/feature/ingest/service"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/secretbox"
)

type Deps struct {
	DB      *postgres.DB
	Config  *config.Config
	Catalog *catalogservice.Service
	Ingest  *ingestservice.Service
	K8s     deploy.ClientFactory
	Secrets *secretbox.Box
}

type Service struct {
	cfg          *config.Config
	db           *postgres.DB
	environments *repository.Environments
	clusters     *repository.Clusters
	workloads    *repository.Workloads
	k8s          deploy.ClientFactory
	secrets      *secretbox.Box
	catalog      *catalogservice.Service
	ingest       *ingestservice.Service
}

func New(d Deps) *Service {
	return &Service{
		cfg:          d.Config,
		db:           d.DB,
		environments: repository.NewEnvironments(d.DB),
		clusters:     repository.NewClusters(d.DB),
		workloads:    repository.NewWorkloads(d.DB),
		k8s:          d.K8s,
		secrets:      d.Secrets,
		catalog:      d.Catalog,
		ingest:       d.Ingest,
	}
}

func (s *Service) ActivitySignals(ctx context.Context, projects []uuid.UUID, _, busy bool) (map[uuid.UUID][]catalog.ProcessSignals, error) {
	return s.clusters.Signals(ctx, projects, busy)
}
