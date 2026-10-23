package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	"svc-registry/internal/feature/topology/repositories"
	"svc-registry/pkg/pgtx"
)

type Deps struct {
	DB      *pgtx.DB
	Catalog *catalogservices.CatalogServices
}

type parts struct {
	catalog              *catalogservices.CatalogServices
	dependencyRepository *repositories.DependencyRepository
}

func newParts(d Deps) *parts {
	return &parts{catalog: d.Catalog, dependencyRepository: repositories.NewDependencyRepository(d.DB)}
}

type TopologyServices struct {
	Records      *RecordsService
	Dependencies *DependenciesService
}

func New(d Deps) *TopologyServices {
	p := newParts(d)
	return &TopologyServices{
		Records:      &RecordsService{resolver: resolver{catalog: p.catalog}, dependencyRepository: p.dependencyRepository},
		Dependencies: &DependenciesService{resolver: resolver{catalog: p.catalog}, dependencyRepository: p.dependencyRepository},
	}
}
