package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/feature/links"
	"svc-registry/internal/platform/apperr"
)

const projectPageSize = 100

func (s *Service) ClaimLinkChecks(ctx context.Context, limit, leaseSecs int) ([]links.Target, error) {
	items, err := s.targets.Claim(ctx, limit, leaseSecs)
	return items, apperr.Wrap(err)
}

func (s *Service) ExtendLinkCheck(ctx context.Context, t links.Target, leaseSecs int) error {
	return apperr.Wrap(s.targets.Extend(ctx, t.ID, leaseSecs))
}

func (s *Service) RunLinkCheck(ctx context.Context, t links.Target) error {
	r := s.checker.Check(ctx, t.URL)
	if ctx.Err() != nil {
		return nil
	}
	cfg := s.cfg.LinkCheck
	_, err := s.targets.Record(ctx, t.URL, r, cfg.IntervalSecs, cfg.History)
	return apperr.Wrap(err)
}

func (s *Service) RefreshLinkTargets(ctx context.Context) (int, error) {
	seen := 0
	after := uuid.Nil
	for {
		ids, err := s.templates.Projects(ctx, after, projectPageSize)
		if err != nil {
			return seen, apperr.Wrap(err)
		}
		for _, id := range ids {
			urls, err := s.projectURLs(ctx, id)
			if err != nil {
				return seen, err
			}
			if err := s.targets.Seen(ctx, urls); err != nil {
				return seen, apperr.Wrap(err)
			}
			seen += len(urls)
		}
		if len(ids) < projectPageSize {
			return seen, nil
		}
		after = ids[len(ids)-1]
	}
}

func (s *Service) projectURLs(ctx context.Context, id uuid.UUID) ([]string, error) {
	chain, err := s.catalog.NodeChain(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if chain == nil || len(chain.Nodes) == 0 {
		return nil, nil
	}
	ancestors := make([]catalog.Node, 0, len(chain.Nodes)-1)
	for i := len(chain.Nodes) - 1; i >= 1; i-- {
		ancestors = append(ancestors, chain.Nodes[i].Node)
	}
	c, templates, err := s.loadContext(ctx, chain.Nodes[0].Node, ancestors)
	if err != nil {
		return nil, err
	}
	return links.URLs(links.Expand(c, templates, "")), nil
}

func (s *Service) PruneLinkTargets(ctx context.Context) (int64, error) {
	n, err := s.targets.Prune(ctx, s.cfg.LinkCheck.TargetTTLDays)
	return n, apperr.Wrap(err)
}
