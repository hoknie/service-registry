package service

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"svc-registry/internal/access"
	"svc-registry/internal/auth"
	"svc-registry/internal/catalog"
	"svc-registry/internal/config"
	"svc-registry/internal/deploy"
	"svc-registry/internal/forge"
	"svc-registry/internal/ingest"
	"svc-registry/internal/knowledge"
	"svc-registry/internal/links"
	"svc-registry/pkg/secretbox"
)

type State struct {
	Config        *config.Config
	DB            *pgxpool.Pool
	Users         access.UserStore
	Sessions      access.SessionStore
	Groups        access.GroupStore
	Tokens        access.TokenStore
	Identities    access.IdentityStore
	LoginStates   access.LoginStateStore
	OAuth         access.OAuthClient
	Nodes         catalog.NodeStore
	Bindings      catalog.BindingStore
	ProjectKeys   catalog.ProjectKeyStore
	Branches      catalog.BranchStore
	Activity      catalog.ActivityStore
	Events        ingest.EventStore
	Deployments   ingest.DeploymentStore
	Connections   forge.ConnectionStore
	Repositories  forge.RepositoryStore
	Runs          forge.RunStore
	Forges        forge.ClientFactory
	LinkKinds     links.KindStore
	LinkTemplates links.TemplateStore
	LinkTargets   links.TargetStore
	LinkChecker   links.Checker
	Environments  deploy.EnvironmentStore
	Clusters      deploy.ClusterStore
	Workloads     deploy.WorkloadStore
	K8s           deploy.ClientFactory
	Knowledge     knowledge.SettingsStore
	Patterns      knowledge.PatternStore
	Snapshots     knowledge.SnapshotStore
	DocSearch     knowledge.SearchStore
	DocIndex      knowledge.IndexStore
	Scans         knowledge.ScanStore
	SearchEngine  knowledge.Engine
	ExternalIndex knowledge.ExternalIndex
	Embedder      knowledge.Embedder
	Sources       knowledge.SourceStore
	DocReaders    knowledge.ReaderFactory
	Secrets       *secretbox.Box
	Hasher        *auth.PasswordHasher
	LoginLimiter  *auth.LoginLimiter
	IngestLimiter *auth.IngestLimiter
}
