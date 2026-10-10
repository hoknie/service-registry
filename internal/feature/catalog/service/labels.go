package service

import (
	"context"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) SuggestLabels(ctx context.Context, p access.Principal, q catalog.LabelQuery) ([]catalog.LabelSuggestion, error) {
	v, err := catalog.ValidateLabelQuery(q)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	prefix := catalog.LikePrefix(v.Q)
	var out []catalog.LabelSuggestion
	if v.Key != nil {
		out, err = s.nodes.LabelValues(ctx, p.UserID, p.IsSuperadmin, *v.Key, prefix)
	} else {
		out, err = s.nodes.LabelKeys(ctx, p.UserID, p.IsSuperadmin, prefix)
	}
	return out, apperr.Wrap(err)
}
