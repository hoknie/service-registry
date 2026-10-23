package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	deployports "svc-registry/internal/feature/deploy/ports"
	"svc-registry/internal/feature/deploy/repositories"
	deploytypes "svc-registry/internal/feature/deploy/types"
	ingestservices "svc-registry/internal/feature/ingest/services"
	notifyservices "svc-registry/internal/feature/notify/services"
	topologyservices "svc-registry/internal/feature/topology/services"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

type Deps struct {
	DB       *pgtx.DB
	Config   *config.Config
	Catalog  *catalogservices.CatalogServices
	Ingest   *ingestservices.IngestServices
	K8s      deployports.ClientFactoryPort
	Secrets  *secretbox.Box
	Notify   *notifyservices.NotifyServices
	Topology *topologyservices.TopologyServices
}

type parts struct {
	cfg                   *config.Config
	db                    *pgtx.DB
	environmentRepository *repositories.EnvironmentRepository
	clusterRepository     *repositories.ClusterRepository
	agentTokenRepository  *repositories.AgentTokenRepository
	workloadRepository    *repositories.WorkloadRepository
	k8s                   deployports.ClientFactoryPort
	secrets               *secretbox.Box
	catalog               *catalogservices.CatalogServices
	ingest                *ingestservices.IngestServices
	notify                *notifyservices.NotifyServices
	topology              *topologyservices.TopologyServices
}

func newParts(d Deps) *parts {
	return &parts{
		cfg:                   d.Config,
		db:                    d.DB,
		environmentRepository: repositories.NewEnvironmentRepository(d.DB),
		clusterRepository:     repositories.NewClusterRepository(d.DB, workloadsPerGapSec(d.Config)),
		agentTokenRepository:  repositories.NewAgentTokenRepository(d.DB),
		workloadRepository:    repositories.NewWorkloadRepository(d.DB),
		k8s:                   d.K8s,
		secrets:               d.Secrets,
		catalog:               d.Catalog,
		ingest:                d.Ingest,
		notify:                d.Notify,
		topology:              d.Topology,
	}
}

func workloadsPerGapSec(cfg *config.Config) int32 {
	if cfg == nil {
		return deploytypes.DefaultWorkloadsPerGapSec
	}
	return cfg.K8s.WorkloadsPerGapSec
}

type DeployServices struct {
	AgentTokens       *AgentTokensService
	Clusters          *ClustersService
	EnvironmentStates *EnvironmentStatesService
	Environments      *EnvironmentsService
	Poll              *PollService
	Ports             *PortsService
	Reports           *ReportsService
}

func New(d Deps) *DeployServices {
	p := newParts(d)
	f := &DeployServices{
		AgentTokens:       &AgentTokensService{agentTokenRepository: p.agentTokenRepository, clusterRepository: p.clusterRepository},
		Clusters:          &ClustersService{topology: p.topology, catalog: p.catalog, clusterRepository: p.clusterRepository, db: p.db, k8s: p.k8s, secrets: p.secrets, workloadRepository: p.workloadRepository},
		EnvironmentStates: &EnvironmentStatesService{catalog: p.catalog, environmentRepository: p.environmentRepository, ingest: p.ingest, workloadRepository: p.workloadRepository},
		Environments:      &EnvironmentsService{environmentRepository: p.environmentRepository},
		Poll:              &PollService{catalog: p.catalog, cfg: p.cfg, clusterRepository: p.clusterRepository, db: p.db, ingest: p.ingest, notify: p.notify, topology: p.topology, workloadRepository: p.workloadRepository},
		Ports:             &PortsService{clusterRepository: p.clusterRepository, secrets: p.secrets},
		Reports:           &ReportsService{agentTokenRepository: p.agentTokenRepository, clusterRepository: p.clusterRepository},
	}
	f.Poll.clustersService = f.Clusters
	f.Clusters.tokens = f.AgentTokens
	f.Reports.poll = f.Poll
	return f
}
