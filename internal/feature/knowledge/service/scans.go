package service

import (
	"context"

	"svc-registry/internal/feature/access"
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
