package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/knowledge"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) ListScans(ctx context.Context, p access.Principal, q knowledge.ScanQuery, page access.PageQuery) (access.Page[knowledge.ScanItem], error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return access.Page[knowledge.ScanItem]{}, err
	}
	filter, err := knowledge.ValidateScanFilter(q)
	if err != nil {
		return access.Page[knowledge.ScanItem]{}, apperr.Wrap(err)
	}
	req, err := access.ValidatePage(page)
	if err != nil {
		return access.Page[knowledge.ScanItem]{}, apperr.Wrap(err)
	}
	out, err := s.scans.List(ctx, filter, req)
	return out, apperr.Wrap(err)
}

func (s *Service) ListProjectScans(ctx context.Context, p access.Principal, id uuid.UUID, q knowledge.ScanQuery, page access.PageQuery) (access.Page[knowledge.ScanItem], error) {
	if err := s.catalog.AuthorizeProject(ctx, p, catalog.PermWrite, id); err != nil {
		return access.Page[knowledge.ScanItem]{}, err
	}
	q.Project = ""
	filter, err := knowledge.ValidateScanFilter(q)
	if err != nil {
		return access.Page[knowledge.ScanItem]{}, apperr.Wrap(err)
	}
	filter.Project = &id
	req, err := access.ValidatePage(page)
	if err != nil {
		return access.Page[knowledge.ScanItem]{}, apperr.Wrap(err)
	}
	out, err := s.scans.List(ctx, filter, req)
	return out, apperr.Wrap(err)
}
