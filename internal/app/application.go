package app

import (
	"context"
	"errors"

	"svc-registry/internal/feature/access/clients/oidc"
	accessservices "svc-registry/internal/feature/access/services"
	apisservices "svc-registry/internal/feature/apis/services"
	catalogports "svc-registry/internal/feature/catalog/ports"
	catalogservices "svc-registry/internal/feature/catalog/services"
	composeservices "svc-registry/internal/feature/compose/services"
	deployservices "svc-registry/internal/feature/deploy/services"
	gatewayservices "svc-registry/internal/feature/gateway/services"
	ingestservices "svc-registry/internal/feature/ingest/services"
	"svc-registry/internal/feature/knowledge/clients/source"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	"svc-registry/internal/feature/links/clients/linkcheck"
	"svc-registry/internal/feature/links/clients/uploads"
	linksports "svc-registry/internal/feature/links/ports"
	linksservices "svc-registry/internal/feature/links/services"
	membershipservices "svc-registry/internal/feature/membership/services"
	notifyservices "svc-registry/internal/feature/notify/services"
	ownershipservices "svc-registry/internal/feature/ownership/services"
	publishingservices "svc-registry/internal/feature/publishing/services"
	"svc-registry/internal/feature/reposync/clients/hostingclient"
	reposyncservices "svc-registry/internal/feature/reposync/services"
	skillsports "svc-registry/internal/feature/skills/ports"
	skillsservices "svc-registry/internal/feature/skills/services"
	topologyservices "svc-registry/internal/feature/topology/services"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/metrics"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/outbound"
	"svc-registry/pkg/password"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

var ErrNotWired = errors.New("catalog ports are not wired")

type App struct {
	Config      *config.Config
	DB          *pgtx.DB
	Hasher      *password.Hasher
	Secrets     *secretbox.Box
	Access      *accessservices.AccessServices
	Catalog     *catalogservices.CatalogServices
	Links       *linksservices.LinksServices
	Ownership   *ownershipservices.OwnershipServices
	Notify      *notifyservices.NotifyServices
	Topology    *topologyservices.TopologyServices
	Ingest      *ingestservices.IngestServices
	RepoSync    *reposyncservices.RepoSyncServices
	Deploy      *deployservices.DeployServices
	Knowledge   *knowledgeservices.KnowledgeServices
	Publishing  *publishingservices.PublishingServices
	Skills      *skillsservices.SkillsServices
	Gateway     *gatewayservices.GatewayServices
	Membership  *membershipservices.MembershipServices
	Apis        *apisservices.ApisServices
	Compose     *composeservices.ComposeServices
	Metrics     *metrics.Metrics
	Sampler     *metrics.Sampler
	search      search
	skillsStore skillsports.BlobStorePort
}

func New(cfg config.Config, opts ...Option) (*App, error) {
	o := overridesOf(opts)
	pool, err := postgres.ConnectLazy(cfg.DB)
	if err != nil {
		return nil, err
	}
	m := newMetrics(cfg)
	srch, err := buildSearch(cfg, pool, m)
	if err != nil {
		pool.Close()
		return nil, err
	}
	db := pgtx.New(pool)
	a := &App{
		Config:  &cfg,
		DB:      db,
		Hasher:  password.NewHasher(cfg.PasswordHash.Params()),
		Secrets: secretbox.New(cfg.Secrets.Keys),
		Metrics: m,
		search:  srch,
	}
	hostings := hostingclient.NewFactoryClient(m.Client("hosting", outbound.New(cfg.Outbound.Options())))
	readers := &source.FactoryClient{Roots: cfg.Knowledge.LocalRoots, Hostings: hostings,
		MaxFileBytes: cfg.Knowledge.MaxFileBytes, MaxBranches: int(cfg.Branches.SyncMaxPerRepo)}
	a.Access = accessservices.New(accessservices.Deps{DB: db, Config: &cfg, Hasher: a.Hasher,
		OAuth:          oidc.NewOIDCClient(cfg.OAuth, cfg.Web.PublicURL, m.Client("oidc", outbound.New(cfg.Outbound.Options()))),
		ClientMetadata: instrument(m, "client_metadata", clientMetadata(o, cfg)), LoginLimiter: auth.NewLoginLimiter(cfg.LoginLimit),
		Metrics: m, Instance: hostname()})
	a.Catalog = catalogservices.New(catalogservices.Deps{DB: db, Config: &cfg, Secrets: a.Secrets})
	a.Ownership = ownershipservices.New(ownershipservices.Deps{DB: db, Catalog: a.Catalog})
	a.Notify = notifyservices.New(notifyservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Ownership: a.Ownership,
		Sender: instrument(m, "notify", notifySender(o, cfg)), Secrets: a.Secrets, Version: Version})
	a.Links = linksservices.New(linksservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Checker: instrument(m, "link_check", linkChecker(o, cfg)),
		Icons: uploads.New(cfg.Uploads.Dir), Notify: a.Notify})
	a.Topology = topologyservices.New(topologyservices.Deps{DB: db, Catalog: a.Catalog})
	a.Ingest = ingestservices.New(ingestservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Limiter: auth.NewIngestLimiter(cfg.Ingest),
		Notify: a.Notify, Topology: a.Topology, Metrics: m})
	a.RepoSync = reposyncservices.New(reposyncservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Clients: hostings, Secrets: a.Secrets,
		Notify: a.Notify})
	a.Deploy = deployservices.New(deployservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Ingest: a.Ingest,
		K8s: instrument(m, "kubernetes", k8sFactory(o, cfg.Outbound, cfg.K8s.K8sClientConfig)), Secrets: a.Secrets, Notify: a.Notify, Topology: a.Topology})
	a.Knowledge = knowledgeservices.New(knowledgeservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, RepoSync: a.RepoSync,
		Engine: srch.engine, External: srch.external, Embedder: srch.embedder, Secrets: a.Secrets, Topology: a.Topology,
		Readers: readers})
	a.Apis = apisservices.New(apisservices.Deps{Knowledge: a.Knowledge})
	a.Publishing = publishingservices.New(publishingservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Knowledge: a.Knowledge, Apis: a.Apis,
		Limiter: auth.NewPublicDocsLimiter(cfg.DocsPublic.RatePerMinute)})
	a.Compose = composeservices.New(composeservices.Deps{Catalog: a.Catalog, RepoSync: a.RepoSync, Knowledge: a.Knowledge, Links: a.Links,
		Ownership: a.Ownership})
	a.Access.Use(a.Catalog.Access)
	a.Access.UseStewards(a.Ownership.Ports)
	a.Access.UseOrganizations(a.Catalog.Access)
	a.Catalog.Use(a.RepoSync.Ports, a.Knowledge.Ports, a.RepoSync.Ports, a.Deploy.Ports)
	if err := a.buildSkills(o, readers); err != nil {
		pool.Close()
		return nil, err
	}
	users := []catalogports.SecretUserPort{a.RepoSync.Ports, a.Knowledge.Ports, a.Deploy.Ports, a.Notify.Ports}
	if a.Skills != nil {
		users = append(users, a.Skills.Ports)
	}
	a.buildGateway(o)
	if a.Gateway != nil {
		users = append(users, a.Gateway.Ports)
		a.Access.UseMCPServers(a.Gateway.Ports)
	}
	a.Catalog.UseSecretUsers(users...)
	a.buildMembership(o)
	a.Access.UseInvitations(a.Membership.Ports)
	a.Catalog.UseOwners(a.Ownership.Ports, a.Ownership.Ports)
	a.Catalog.UseTokenRoots(a.Access.Tokens)
	a.RepoSync.Use(a.Knowledge.Sources, a.Knowledge.Ports)
	a.Ingest.Use(a.Knowledge.Ports)
	a.Sampler = a.newSampler(o.metricSources)
	if !a.Catalog.Wired() {
		pool.Close()
		return nil, ErrNotWired
	}
	return a, nil
}

func (a *App) Close() { a.DB.Pool().Close() }

func (a *App) CheckSearch(ctx context.Context) error { return a.search.check(ctx) }

func linkChecker(o overrides, cfg config.Config) linksports.CheckerPort {
	if o.linkChecker != nil {
		return o.linkChecker
	}
	return linkcheck.New(cfg.Outbound, cfg.LinkCheck)
}
