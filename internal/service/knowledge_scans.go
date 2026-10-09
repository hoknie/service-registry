package service

import (
	"context"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/knowledge"
)

func ListScans(ctx context.Context, state *State, p Principal, q knowledge.ScanQuery, page access.PageQuery) (access.Page[knowledge.ScanItem], error) {
	if err := RequireSuperadmin(p); err != nil {
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
	out, err := state.Scans.List(ctx, filter, req)
	return out, apperr.Wrap(err)
}
