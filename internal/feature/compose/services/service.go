package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
	linksservices "svc-registry/internal/feature/links/services"
	ownershipservices "svc-registry/internal/feature/ownership/services"
	reposyncservices "svc-registry/internal/feature/reposync/services"
)

type Deps struct {
	Catalog   *catalogservices.CatalogServices
	RepoSync  *reposyncservices.RepoSyncServices
	Knowledge *knowledgeservices.KnowledgeServices
	Links     *linksservices.LinksServices
	Ownership *ownershipservices.OwnershipServices
}

type parts struct {
	catalog   *catalogservices.CatalogServices
	repoSync  *reposyncservices.RepoSyncServices
	knowledge *knowledgeservices.KnowledgeServices
	links     *linksservices.LinksServices
	ownership *ownershipservices.OwnershipServices
}

func newParts(d Deps) *parts {
	return &parts{catalog: d.Catalog, repoSync: d.RepoSync, knowledge: d.Knowledge, links: d.Links, ownership: d.Ownership}
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
		MCP:      &MCPService{catalog: p.catalog, repoSync: p.repoSync, knowledge: p.knowledge, ownership: p.ownership},
		Projects: &ProjectsService{catalog: p.catalog, repoSync: p.repoSync, knowledge: p.knowledge},
	}
	return f
}
