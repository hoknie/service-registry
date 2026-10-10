package http

import (
	accessservice "svc-registry/internal/feature/access/service"
	catalogservice "svc-registry/internal/feature/catalog/service"
	deployservice "svc-registry/internal/feature/deploy/service"
	forgeservice "svc-registry/internal/feature/forge/service"
	ingestservice "svc-registry/internal/feature/ingest/service"
	knowledgeservice "svc-registry/internal/feature/knowledge/service"
	linksservice "svc-registry/internal/feature/links/service"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/postgres"
	"svc-registry/internal/presentation/http/handlers"
)

type Deps struct {
	Config    *config.Config
	DB        *postgres.DB
	Version   string
	Access    *accessservice.Service
	Catalog   *catalogservice.Service
	Links     *linksservice.Service
	Ingest    *ingestservice.Service
	Forge     *forgeservice.Service
	Deploy    *deployservice.Service
	Knowledge *knowledgeservice.Service
}

func (d Deps) handlers() *handlers.Handlers {
	return &handlers.Handlers{Config: d.Config, DB: d.DB, Version: d.Version, Access: d.Access, Catalog: d.Catalog, Links: d.Links,
		Ingest: d.Ingest, Forge: d.Forge, Deploy: d.Deploy, Knowledge: d.Knowledge}
}
