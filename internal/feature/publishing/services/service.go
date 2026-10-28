package services

import (
	apisservices "svc-registry/internal/feature/apis/services"
	catalogservices "svc-registry/internal/feature/catalog/services"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	"svc-registry/internal/feature/publishing/repositories"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

type Deps struct {
	Limiter   *auth.PublicDocsLimiter
	DB        *pgtx.DB
	Config    *config.Config
	Catalog   *catalogservices.CatalogServices
	Knowledge *knowledgeservices.KnowledgeServices
	Apis      *apisservices.ApisServices
}

type PublishingServices struct {
	Settings *SettingsService
	Public   *PublicService
	Hosts    *HostsService
}

func New(d Deps) *PublishingServices {
	publications := repositories.NewPublicationRepository(d.DB)
	hosts := &HostsService{publicationRepository: publications}
	return &PublishingServices{
		Settings: &SettingsService{db: d.DB, cfg: d.Config, catalog: d.Catalog, publicationRepository: publications, hosts: hosts},
		Public:   &PublicService{knowledge: d.Knowledge, apis: d.Apis, publicationRepository: publications, hosts: hosts, limiter: d.Limiter},
		Hosts:    hosts,
	}
}
