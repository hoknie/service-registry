package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	pollingports "svc-registry/internal/feature/polling/ports"
	"svc-registry/internal/feature/polling/repositories"
	pollingtypes "svc-registry/internal/feature/polling/types"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

type Deps struct {
	DB      *pgtx.DB
	Config  *config.Config
	Catalog *catalogservices.CatalogServices
}

type PollingServices struct {
	Settings *SettingsService
	Ports    *PortsService
}

func New(d Deps) *PollingServices {
	polling := repositories.NewPollingRepository(d.DB)
	ports := &PortsService{pollingRepository: polling, defaults: defaultsOf(d.Config)}
	return &PollingServices{
		Settings: &SettingsService{db: d.DB, catalog: d.Catalog, pollingRepository: polling, ports: ports},
		Ports:    ports,
	}
}

func (s *PollingServices) Use(replanners ...pollingports.ReplanPort) {
	s.Settings.replanners = append(s.Settings.replanners, replanners...)
}

func defaultsOf(cfg *config.Config) pollingtypes.Defaults {
	return pollingtypes.Defaults{RepoSyncIntervalSecs: int(cfg.Jobs.RepoSyncIntervalSecs), DocsIntervalSecs: int(cfg.Knowledge.IntervalSecs),
		SkillsIntervalSecs: int(cfg.Skills.SyncIntervalSecs), TimeoutSecs: int(cfg.Outbound.TimeoutSecs)}
}
