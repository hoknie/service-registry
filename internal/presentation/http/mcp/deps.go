package mcp

import (
	catalogservice "svc-registry/internal/feature/catalog/service"
	deployservice "svc-registry/internal/feature/deploy/service"
	forgeservice "svc-registry/internal/feature/forge/service"
	knowledgeservice "svc-registry/internal/feature/knowledge/service"
	linksservice "svc-registry/internal/feature/links/service"
)

type Deps struct {
	Catalog        *catalogservice.Service
	Deploy         *deployservice.Service
	Forge          *forgeservice.Service
	Knowledge      *knowledgeservice.Service
	Links          *linksservice.Service
	MaxResultBytes int
}
