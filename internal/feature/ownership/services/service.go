package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	"svc-registry/internal/feature/ownership/repositories"
	"svc-registry/pkg/pgtx"
)

type Deps struct {
	DB      *pgtx.DB
	Catalog *catalogservices.CatalogServices
}

type parts struct {
	catalog         *catalogservices.CatalogServices
	ownerRepository *repositories.OwnerRepository
}

func newParts(d Deps) *parts {
	return &parts{catalog: d.Catalog, ownerRepository: repositories.NewOwnerRepository(d.DB)}
}

type OwnershipServices struct {
	Owners *OwnersService
	Ports  *PortsService
}

func New(d Deps) *OwnershipServices {
	p := newParts(d)
	return &OwnershipServices{
		Owners: &OwnersService{catalog: p.catalog, ownerRepository: p.ownerRepository},
		Ports:  &PortsService{catalog: p.catalog, ownerRepository: p.ownerRepository},
	}
}
