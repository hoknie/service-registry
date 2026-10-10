package jobs

import (
	accessservice "svc-registry/internal/feature/access/service"
	catalogservice "svc-registry/internal/feature/catalog/service"
	deployservice "svc-registry/internal/feature/deploy/service"
	forgeservice "svc-registry/internal/feature/forge/service"
	ingestservice "svc-registry/internal/feature/ingest/service"
	knowledgeservice "svc-registry/internal/feature/knowledge/service"
	linksservice "svc-registry/internal/feature/links/service"
	"svc-registry/internal/platform/config"
)

type Deps struct {
	Config    *config.Config
	Access    *accessservice.Service
	Catalog   *catalogservice.Service
	Links     *linksservice.Service
	Ingest    *ingestservice.Service
	Forge     *forgeservice.Service
	Deploy    *deployservice.Service
	Knowledge *knowledgeservice.Service
}
