package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	linksservices "svc-registry/internal/feature/links/services"
	linkstypes "svc-registry/internal/feature/links/types"
	ownershipservices "svc-registry/internal/feature/ownership/services"
	ownershiptypes "svc-registry/internal/feature/ownership/types"
	"svc-registry/internal/platform/apperr"
)

type CatalogTable struct {
	Page   catalogservices.CatalogTablePage
	Badges map[uuid.UUID][]linkstypes.Badge
	Owners map[uuid.UUID][]ownershiptypes.Badge
}

func (s *CatalogService) CatalogTable(ctx context.Context, p accesstypes.Principal, parent *uuid.UUID, pq accesstypes.PageQuery, tq catalogtypes.TableQuery) (CatalogTable, error) {
	page, err := s.catalog.Table.CatalogTable(ctx, p, parent, pq, tq)
	if err != nil {
		return CatalogTable{}, apperr.Wrap(err)
	}
	badges, err := s.links.Projects.ProjectBadges(ctx, page.ReadableProjects())
	if err != nil {
		return CatalogTable{}, apperr.Wrap(err)
	}
	owners, err := s.ownership.Owners.OwnerBadges(ctx, page.ReadableNodes())
	if err != nil {
		return CatalogTable{}, apperr.Wrap(err)
	}
	return CatalogTable{Page: page, Badges: badges, Owners: owners}, nil
}

type CatalogService struct {
	catalog   *catalogservices.CatalogServices
	links     *linksservices.LinksServices
	ownership *ownershipservices.OwnershipServices
}
