package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/ingest"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) ListEvents(ctx context.Context, p access.Principal, id uuid.UUID, f ingest.EventFilter, q access.PageQuery) (access.Page[ingest.Event], error) {
	if err := s.catalog.AuthorizeProject(ctx, p, catalog.PermRead, id); err != nil {
		return access.Page[ingest.Event]{}, err
	}
	page, err := access.ValidatePage(q)
	if err != nil {
		return access.Page[ingest.Event]{}, apperr.Wrap(err)
	}
	out, err := s.events.List(ctx, id, f, page)
	return out, apperr.Wrap(err)
}

func (s *Service) ListDeployments(ctx context.Context, p access.Principal, id uuid.UUID, f ingest.DeploymentFilter, q access.PageQuery) (access.Page[ingest.Deployment], error) {
	if err := s.catalog.AuthorizeProject(ctx, p, catalog.PermRead, id); err != nil {
		return access.Page[ingest.Deployment]{}, err
	}
	page, err := access.ValidatePage(q)
	if err != nil {
		return access.Page[ingest.Deployment]{}, apperr.Wrap(err)
	}
	out, err := s.deployments.List(ctx, id, f, page)
	return out, apperr.Wrap(err)
}
