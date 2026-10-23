package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	forgeservices "svc-registry/internal/feature/forge/services"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	linksservices "svc-registry/internal/feature/links/services"
	ownershipservices "svc-registry/internal/feature/ownership/services"
)

type Deps struct {
	Catalog   *catalogservices.CatalogServices
	Forge     *forgeservices.ForgeServices
	Knowledge *knowledgeservices.KnowledgeServices
	Links     *linksservices.LinksServices
	Ownership *ownershipservices.OwnershipServices
}

type parts struct {
	catalog   *catalogservices.CatalogServices
	forge     *forgeservices.ForgeServices
	knowledge *knowledgeservices.KnowledgeServices
	links     *linksservices.LinksServices
	ownership *ownershipservices.OwnershipServices
}

func newParts(d Deps) *parts {
	return &parts{catalog: d.Catalog, forge: d.Forge, knowledge: d.Knowledge, links: d.Links, ownership: d.Ownership}
}

type ComposeServices struct {
	Catalog  *CatalogService
	MCP      *MCPService
	Projects *ProjectsService
}

func New(d Deps) *ComposeServices {
	p := newParts(d)
	f := &ComposeServices{
		Catalog:  &CatalogService{catalog: p.catalog, links: p.links, ownership: p.ownership},
		MCP:      &MCPService{catalog: p.catalog, forge: p.forge, knowledge: p.knowledge, ownership: p.ownership},
		Projects: &ProjectsService{catalog: p.catalog, forge: p.forge, knowledge: p.knowledge},
	}
	return f
}
