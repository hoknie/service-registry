package app

import (
	"context"
	"errors"

	"svc-registry/internal/feature/access/oidc"
	accessservice "svc-registry/internal/feature/access/service"
	catalogservice "svc-registry/internal/feature/catalog/service"
	deployservice "svc-registry/internal/feature/deploy/service"
	"svc-registry/internal/feature/forge/forgeclient"
	forgeservice "svc-registry/internal/feature/forge/service"
	ingestservice "svc-registry/internal/feature/ingest/service"
	knowledgeservice "svc-registry/internal/feature/knowledge/service"
	"svc-registry/internal/feature/knowledge/source"
	"svc-registry/internal/feature/links"
	"svc-registry/internal/feature/links/linkcheck"
	linksservice "svc-registry/internal/feature/links/service"
	"svc-registry/internal/feature/links/uploads"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/outbound"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/secretbox"
)

var ErrNotWired = errors.New("catalog ports are not wired")

type App struct {
	Config    *config.Config
	DB        *postgres.DB
	Hasher    *auth.PasswordHasher
	Secrets   *secretbox.Box
	Access    *accessservice.Service
	Catalog   *catalogservice.Service
	Links     *linksservice.Service
	Ingest    *ingestservice.Service
	Forge     *forgeservice.Service
	Deploy    *deployservice.Service
	Knowledge *knowledgeservice.Service
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
	db := postgres.New(pool)
	a := &App{
		Config:  &cfg,
		DB:      db,
		Hasher:  auth.NewPasswordHasher(cfg.PasswordHash),
		Secrets: secretbox.New(cfg.Secrets.Keys),
		search:  srch,
	}
	forges := forgeclient.NewFactory(outbound.New(cfg.Outbound))
	a.Access = accessservice.New(accessservice.Deps{DB: db, Config: &cfg, Hasher: a.Hasher,
		OAuth:        oidc.NewOIDC(cfg.OAuth, cfg.Web.PublicURL, outbound.New(cfg.Outbound)),
		LoginLimiter: auth.NewLoginLimiter(cfg.LoginLimit)})
	a.Catalog = catalogservice.New(catalogservice.Deps{DB: db, Config: &cfg, Secrets: a.Secrets})
	a.Links = linksservice.New(linksservice.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Checker: linkChecker(o, cfg),
		Icons: uploads.New(cfg.Uploads.Dir)})
	a.Ingest = ingestservice.New(ingestservice.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Limiter: auth.NewIngestLimiter(cfg.Ingest)})
	a.Forge = forgeservice.New(forgeservice.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Clients: forges, Secrets: a.Secrets})
	a.Deploy = deployservice.New(deployservice.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Ingest: a.Ingest,
		K8s: k8sFactory(o, cfg), Secrets: a.Secrets})
	a.Knowledge = knowledgeservice.New(knowledgeservice.Deps{DB: db, Config: &cfg, Catalog: a.Catalog, Forge: a.Forge,
		Engine: srch.engine, External: srch.external, Embedder: srch.embedder, Secrets: a.Secrets,
		Readers: &source.Factory{Roots: cfg.Knowledge.LocalRoots, Forges: forges,
			MaxFileBytes: cfg.Knowledge.MaxFileBytes, MaxBranches: int(cfg.Branches.SyncMaxPerRepo)}})
	a.Catalog.Use(a.Forge, a.Knowledge, a.Forge, a.Deploy)
	a.Catalog.UseSecretUsers(a.Forge, a.Knowledge, a.Deploy)
	if !a.Catalog.Wired() {
		pool.Close()
		return nil, ErrNotWired
	}
	return a, nil
}

func (a *App) Close() { a.DB.Pool().Close() }

func (a *App) CheckSearch(ctx context.Context) error { return a.search.check(ctx) }

func linkChecker(o overrides, cfg config.Config) links.Checker {
	if o.linkChecker != nil {
		return o.linkChecker
	}
	return linkcheck.New(cfg.Outbound, cfg.LinkCheck)
}
