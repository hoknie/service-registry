package app

import (
	"context"
	"errors"

	"svc-registry/internal/feature/access/clients/oidc"
	accessservices "svc-registry/internal/feature/access/services"
	apisservices "svc-registry/internal/feature/apis/services"
	catalogservices "svc-registry/internal/feature/catalog/services"
	composeservices "svc-registry/internal/feature/compose/services"
	deployservices "svc-registry/internal/feature/deploy/services"
	"svc-registry/internal/feature/forge/clients/forgeclient"
	forgeservices "svc-registry/internal/feature/forge/services"
	ingestservices "svc-registry/internal/feature/ingest/services"
	"svc-registry/internal/feature/knowledge/clients/source"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	"svc-registry/internal/feature/links/clients/linkcheck"
	"svc-registry/internal/feature/links/clients/uploads"
	linksports "svc-registry/internal/feature/links/ports"
	linksservices "svc-registry/internal/feature/links/services"
	notifyservices "svc-registry/internal/feature/notify/services"
	ownershipservices "svc-registry/internal/feature/ownership/services"
	topologyservices "svc-registry/internal/feature/topology/services"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/outbound"
	"svc-registry/pkg/password"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

var ErrNotWired = errors.New("catalog ports are not wired")

type App struct {
	Config    *config.Config
	DB        *pgtx.DB
	Hasher    *password.Hasher
	Secrets   *secretbox.Box
	Access    *accessservices.AccessServices
	Catalog   *catalogservices.CatalogServices
	Links     *linksservices.LinksServices
	Ownership *ownershipservices.OwnershipServices
	Notify    *notifyservices.NotifyServices
	Topology  *topologyservices.TopologyServices
	Ingest    *ingestservices.IngestServices
	Forge     *forgeservices.ForgeServices
	Deploy    *deployservices.DeployServices
	Knowledge *knowledgeservices.KnowledgeServices
	Apis      *apisservices.ApisServices
	Compose   *composeservices.ComposeServices
	search    search
}

func New(cfg config.Config, opts ...Option) (*App, error) {
	o := overridesOf(opts)
	pool, err := postgres.ConnectLazy(cfg.DB)
	if err != nil {
		return nil, err
	}
	srch, err := buildSearch(cfg, pool)
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
		search:  srch,
	}
	forges := forgeclient.NewFactoryClient(outbound.New(cfg.Outbound.Options()))
	a.Access = accessservices.New(accessservices.Deps{DB: db, Config: &cfg, Hasher: a.Hasher,
		OAuth:          oidc.NewOIDCClient(cfg.OAuth, cfg.Web.PublicURL, outbound.New(cfg.Outbound.Options())),
		ClientMetadata: clientMetadata(o, cfg), LoginLimiter: auth.NewLoginLimiter(cfg.LoginLimit)})
	a.Catalog = catalogservices.New(catalogservices.Deps{DB: db, Config: &cfg, Secrets: a.Secrets})
	a.Ownership = ownershipservices.New(ownershipservices.Deps{DB: db, Catalog: a.Catalog})
	a.Notify = notifyservices.New(notifyservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Ownership: a.Ownership,
		Sender: notifySender(o, cfg), Secrets: a.Secrets, Version: Version})
	a.Links = linksservices.New(linksservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Checker: linkChecker(o, cfg),
		Icons: uploads.New(cfg.Uploads.Dir), Notify: a.Notify})
	a.Topology = topologyservices.New(topologyservices.Deps{DB: db, Catalog: a.Catalog})
	a.Ingest = ingestservices.New(ingestservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Limiter: auth.NewIngestLimiter(cfg.Ingest),
		Notify: a.Notify, Topology: a.Topology})
	a.Forge = forgeservices.New(forgeservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Clients: forges, Secrets: a.Secrets,
		Notify: a.Notify})
	a.Deploy = deployservices.New(deployservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Ingest: a.Ingest,
		K8s: k8sFactory(o, cfg.Outbound, cfg.K8s.K8sClientConfig), Secrets: a.Secrets, Notify: a.Notify, Topology: a.Topology})
	a.Knowledge = knowledgeservices.New(knowledgeservices.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Forge: a.Forge,
		Engine: srch.engine, External: srch.external, Embedder: srch.embedder, Secrets: a.Secrets, Topology: a.Topology,
		Readers: &source.FactoryClient{Roots: cfg.Knowledge.LocalRoots, Forges: forges,
			MaxFileBytes: cfg.Knowledge.MaxFileBytes, MaxBranches: int(cfg.Branches.SyncMaxPerRepo)}})
	a.Apis = apisservices.New(apisservices.Deps{Knowledge: a.Knowledge})
	a.Compose = composeservices.New(composeservices.Deps{Catalog: a.Catalog, Forge: a.Forge, Knowledge: a.Knowledge, Links: a.Links,
		Ownership: a.Ownership})
	a.Access.Use(a.Catalog.Access)
	a.Catalog.Use(a.Forge.Ports, a.Knowledge.Ports, a.Forge.Ports, a.Deploy.Ports)
	a.Catalog.UseSecretUsers(a.Forge.Ports, a.Knowledge.Ports, a.Deploy.Ports, a.Notify.Ports)
	a.Catalog.UseOwners(a.Ownership.Ports)
	a.Forge.Use(a.Knowledge.Sources)
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
