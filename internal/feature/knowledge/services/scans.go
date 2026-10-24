package services

import (
	"context"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
)

func (s *ScansService) ListScans(ctx context.Context, p accesstypes.Principal, q knowledgetypes.ScanQuery, page accesstypes.PageQuery) (accesstypes.Page[knowledgetypes.ScanItem], error) {
	if err := accesstypes.RequireRight(p, accesstypes.SectionScans, accesstypes.ActionRead); err != nil {
		return accesstypes.Page[knowledgetypes.ScanItem]{}, err
	}
	filter, err := knowledgetypes.ValidateScanFilter(q)
	if err != nil {
		return accesstypes.Page[knowledgetypes.ScanItem]{}, apperr.Wrap(err)
	}
	req, err := accesstypes.ValidatePage(page)
	if err != nil {
		return accesstypes.Page[knowledgetypes.ScanItem]{}, apperr.Wrap(err)
	}
	out, err := s.scanRepository.List(ctx, filter, req)
	return out, apperr.Wrap(err)
}

func (s *ScansService) ListProjectScans(ctx context.Context, p accesstypes.Principal, id uuid.UUID, q knowledgetypes.ScanQuery, page accesstypes.PageQuery) (accesstypes.Page[knowledgetypes.ScanItem], error) {
	if err := s.catalog.Branches.AuthorizeProject(ctx, p, catalogtypes.PermWrite, id); err != nil {
		return accesstypes.Page[knowledgetypes.ScanItem]{}, err
	}
	q.Project = ""
	filter, err := knowledgetypes.ValidateScanFilter(q)
	if err != nil {
		return accesstypes.Page[knowledgetypes.ScanItem]{}, apperr.Wrap(err)
	}
	filter.Project = &id
	req, err := accesstypes.ValidatePage(page)
	if err != nil {
		return accesstypes.Page[knowledgetypes.ScanItem]{}, apperr.Wrap(err)
	}
	out, err := s.scanRepository.List(ctx, filter, req)
	return out, apperr.Wrap(err)
}

type ScansService struct {
	catalog        *catalogservices.CatalogServices
	scanRepository *repositories.ScanRepository
}
