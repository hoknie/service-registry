package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	"svc-registry/internal/feature/ownership/repositories"
	"svc-registry/internal/feature/ownership/usecases"
	"svc-registry/pkg/pgtx"
)

type Deps struct {
	DB      *pgtx.DB
	Catalog *catalogservices.CatalogServices
}

type parts struct {
	catalog         *catalogservices.CatalogServices
	ownerRepository *repositories.OwnerRepository
	replaceOwners   *usecases.ReplaceOwnersUseCase
}

func newParts(d Deps) *parts {
	owners := repositories.NewOwnerRepository(d.DB)
	return &parts{catalog: d.Catalog, ownerRepository: owners, replaceOwners: usecases.NewReplaceOwners(d.DB, owners)}
}

type OwnershipServices struct {
	Owners *OwnersService
	Ports  *PortsService
}

func New(d Deps) *OwnershipServices {
	p := newParts(d)
	return &OwnershipServices{
		Owners: &OwnersService{catalog: p.catalog, ownerRepository: p.ownerRepository, replaceOwners: p.replaceOwners},
		Ports:  &PortsService{catalog: p.catalog, ownerRepository: p.ownerRepository},
	}
}
