package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	deployports "svc-registry/internal/feature/deploy/ports"
	"svc-registry/internal/feature/deploy/repositories"
	ingestservices "svc-registry/internal/feature/ingest/services"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/secretbox"
)

type Deps struct {
	DB      *postgres.DB
	Config  *config.Config
	Catalog *catalogservices.CatalogServices
	Ingest  *ingestservices.IngestServices
	K8s     deployports.ClientFactoryPort
	Secrets *secretbox.Box
}

type parts struct {
	cfg                   *config.Config
	db                    *postgres.DB
	environmentRepository *repositories.EnvironmentRepository
	clusterRepository     *repositories.ClusterRepository
	workloadRepository    *repositories.WorkloadRepository
	k8s                   deployports.ClientFactoryPort
	secrets               *secretbox.Box
	catalog               *catalogservices.CatalogServices
	ingest                *ingestservices.IngestServices
}

func newParts(d Deps) *parts {
	return &parts{
		cfg:                   d.Config,
		db:                    d.DB,
		environmentRepository: repositories.NewEnvironmentRepository(d.DB),
		clusterRepository:     repositories.NewClusterRepository(d.DB),
		workloadRepository:    repositories.NewWorkloadRepository(d.DB),
		k8s:                   d.K8s,
		secrets:               d.Secrets,
		catalog:               d.Catalog,
		ingest:                d.Ingest,
	}
}

type DeployServices struct {
	Clusters          *ClustersService
	EnvironmentStates *EnvironmentStatesService
	Environments      *EnvironmentsService
	Poll              *PollService
	Ports             *PortsService
}

func New(d Deps) *DeployServices {
	p := newParts(d)
	f := &DeployServices{
		Clusters:          &ClustersService{catalog: p.catalog, clusterRepository: p.clusterRepository, k8s: p.k8s, secrets: p.secrets, workloadRepository: p.workloadRepository},
		EnvironmentStates: &EnvironmentStatesService{catalog: p.catalog, environmentRepository: p.environmentRepository, ingest: p.ingest, workloadRepository: p.workloadRepository},
		Environments:      &EnvironmentsService{environmentRepository: p.environmentRepository},
		Poll:              &PollService{catalog: p.catalog, cfg: p.cfg, clusterRepository: p.clusterRepository, db: p.db, ingest: p.ingest, workloadRepository: p.workloadRepository},
		Ports:             &PortsService{clusterRepository: p.clusterRepository, secrets: p.secrets},
	}
	f.Poll.clustersService = f.Clusters
	return f
}
