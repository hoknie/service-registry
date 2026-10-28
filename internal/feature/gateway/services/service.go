package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	gatewayports "svc-registry/internal/feature/gateway/ports"
	"svc-registry/internal/feature/gateway/repositories"
	"svc-registry/internal/feature/gateway/usecases"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

type Deps struct {
	DB       *pgtx.DB
	Config   *config.Config
	Catalog  *catalogservices.CatalogServices
	Client   gatewayports.ClientPort
	Observer usecases.Observer
}

type GatewayServices struct {
	Folders    *FoldersService
	Servers    *ServersService
	Checks     *ChecksService
	Visibility *VisibilityService
	Proxy      *ProxyService
	Spaces     *SpacesService
	Gateway    *GatewayService
	Ports      *PortsService
}

type parts struct {
	db            *pgtx.DB
	cfg           config.MCPGatewayConfig
	access        *accessRules
	catalog       *catalogservices.CatalogServices
	folders       *repositories.FolderRepository
	servers       *repositories.ServerRepository
	checks        *repositories.CheckRepository
	accessLists   *repositories.AccessRepository
	organizations *repositories.OrganizationRepository
	proxies       *repositories.ProxyRepository
	calls         *repositories.CallRepository
	sets          *repositories.SetRepository
	targets       *targets
}

func New(d Deps) *GatewayServices {
	p := parts{db: d.DB, cfg: d.Config.MCPGateway, access: &accessRules{catalog: d.Catalog}, catalog: d.Catalog,
		folders: repositories.NewFolderRepository(d.DB), servers: repositories.NewServerRepository(d.DB),
		checks: repositories.NewCheckRepository(d.DB), accessLists: repositories.NewAccessRepository(d.DB),
		organizations: repositories.NewOrganizationRepository(d.DB), proxies: repositories.NewProxyRepository(d.DB),
		calls: repositories.NewCallRepository(d.DB), sets: repositories.NewSetRepository(d.DB)}
	p.targets = &targets{proxies: p.proxies, catalog: d.Catalog, cfg: p.cfg}
	check := usecases.NewCheckServer(usecases.CheckServerDeps{Checks: p.checks, Client: d.Client, Observer: d.Observer,
		Interval: int(p.cfg.CheckIntervalSecs)})
	call := usecases.NewCallTool(usecases.CallToolDeps{Calls: p.calls, Checks: p.checks, Client: d.Client, Observer: d.Observer})
	return &GatewayServices{
		Folders:    &FoldersService{parts: p},
		Servers:    &ServersService{parts: p, builtin: repositories.NewBuiltinRepository(d.DB)},
		Checks:     &ChecksService{parts: p, check: check},
		Visibility: &VisibilityService{parts: p},
		Proxy:      &ProxyService{parts: p},
		Spaces:     &SpacesService{parts: p},
		Gateway:    &GatewayService{parts: p, call: call},
		Ports:      &PortsService{parts: p},
	}
}
