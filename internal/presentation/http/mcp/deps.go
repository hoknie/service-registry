package mcp

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	composeservices "svc-registry/internal/feature/compose/services"
	deployservices "svc-registry/internal/feature/deploy/services"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	linksservices "svc-registry/internal/feature/links/services"
)

type Deps struct {
	Catalog        *catalogservices.CatalogServices
	Deploy         *deployservices.DeployServices
	Knowledge      *knowledgeservices.KnowledgeServices
	Links          *linksservices.LinksServices
	Compose        *composeservices.ComposeServices
	MaxResultBytes int
}
