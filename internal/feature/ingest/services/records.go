package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/ingest/repositories"
	ingesttypes "svc-registry/internal/feature/ingest/types"
	"svc-registry/internal/platform/apperr"
)

func (s *RecordsService) ListEvents(ctx context.Context, p accesstypes.Principal, id uuid.UUID, f ingesttypes.EventFilter, q accesstypes.PageQuery) (accesstypes.Page[ingesttypes.Event], error) {
	if err := s.catalog.Branches.AuthorizeProject(ctx, p, catalogtypes.PermRead, id); err != nil {
		return accesstypes.Page[ingesttypes.Event]{}, err
	}
	page, err := accesstypes.ValidatePage(q)
	if err != nil {
		return accesstypes.Page[ingesttypes.Event]{}, apperr.Wrap(err)
	}
	out, err := s.eventRepository.List(ctx, id, f, page)
	return out, apperr.Wrap(err)
}

func (s *RecordsService) ListDeployments(ctx context.Context, p accesstypes.Principal, id uuid.UUID, f ingesttypes.DeploymentFilter, q accesstypes.PageQuery) (accesstypes.Page[ingesttypes.Deployment], error) {
	if err := s.catalog.Branches.AuthorizeProject(ctx, p, catalogtypes.PermRead, id); err != nil {
		return accesstypes.Page[ingesttypes.Deployment]{}, err
	}
	page, err := accesstypes.ValidatePage(q)
	if err != nil {
		return accesstypes.Page[ingesttypes.Deployment]{}, apperr.Wrap(err)
	}
	out, err := s.deploymentRepository.List(ctx, id, f, page)
	return out, apperr.Wrap(err)
}

type RecordsService struct {
	catalog              *catalogservices.CatalogServices
	deploymentRepository *repositories.DeploymentRepository
	eventRepository      *repositories.EventRepository
}
