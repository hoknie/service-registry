package http

import (
	accessservices "svc-registry/internal/feature/access/services"
	catalogservices "svc-registry/internal/feature/catalog/services"
	composeservices "svc-registry/internal/feature/compose/services"
	deployservices "svc-registry/internal/feature/deploy/services"
	forgeservices "svc-registry/internal/feature/forge/services"
	ingestservices "svc-registry/internal/feature/ingest/services"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	linksservices "svc-registry/internal/feature/links/services"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/presentation/http/handlers"
)

type Deps struct {
	Config    *config.Config
	Readiness handlers.Readiness
	Version   string
	Access    *accessservices.AccessServices
	Catalog   *catalogservices.CatalogServices
	Links     *linksservices.LinksServices
	Ingest    *ingestservices.IngestServices
	Forge     *forgeservices.ForgeServices
	Deploy    *deployservices.DeployServices
	Knowledge *knowledgeservices.KnowledgeServices
	Compose   *composeservices.ComposeServices
}

func (d Deps) handlers() *handlers.Handlers {
	return &handlers.Handlers{Config: d.Config, Readiness: d.Readiness, Version: d.Version, Access: d.Access, Catalog: d.Catalog, Links: d.Links,
		Ingest: d.Ingest, Forge: d.Forge, Deploy: d.Deploy, Knowledge: d.Knowledge, Compose: d.Compose}
}
