package services

import (
	"context"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	linksports "svc-registry/internal/feature/links/ports"
	"svc-registry/internal/feature/links/repositories"
	linkstypes "svc-registry/internal/feature/links/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
)

const projectPageSize = 100

func (s *ChecksService) ClaimLinkChecks(ctx context.Context, limit, leaseSecs int) ([]linkstypes.Target, error) {
	items, err := s.targetRepository.Claim(ctx, limit, leaseSecs)
	return items, apperr.Wrap(err)
}

func (s *ChecksService) ExtendLinkCheck(ctx context.Context, t linkstypes.Target, leaseSecs int) error {
	return apperr.Wrap(s.targetRepository.Extend(ctx, t.ID, leaseSecs))
}

func (s *ChecksService) RunLinkCheck(ctx context.Context, t linkstypes.Target) error {
	r := s.checker.Check(ctx, t.URL)
	if ctx.Err() != nil {
		return nil
	}
	cfg := s.cfg.LinkCheck
	_, err := s.targetRepository.Record(ctx, t.URL, r, cfg.IntervalSecs, cfg.History)
	return apperr.Wrap(err)
}

func (s *ChecksService) RefreshLinkTargets(ctx context.Context) (int, error) {
	seen := 0
	after := uuid.Nil
	for {
		ids, err := s.templateRepository.Projects(ctx, after, projectPageSize)
		if err != nil {
			return seen, apperr.Wrap(err)
		}
		for _, id := range ids {
			urls, err := s.projectURLs(ctx, id)
			if err != nil {
				return seen, err
			}
			if err := s.targetRepository.Seen(ctx, urls); err != nil {
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

func (s *ChecksService) projectURLs(ctx context.Context, id uuid.UUID) ([]string, error) {
	chain, err := s.catalog.System.NodeChain(ctx, id)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if chain == nil || len(chain.Nodes) == 0 {
		return nil, nil
	}
	ancestors := make([]catalogtypes.Node, 0, len(chain.Nodes)-1)
	for i := len(chain.Nodes) - 1; i >= 1; i-- {
		ancestors = append(ancestors, chain.Nodes[i].Node)
	}
	c, templates, err := s.projects.loadContext(ctx, chain.Nodes[0].Node, ancestors)
	if err != nil {
		return nil, err
	}
	return linkstypes.URLs(linkstypes.ExpandProject(c, templates, "")), nil
}

func (s *ChecksService) PruneLinkTargets(ctx context.Context) (int64, error) {
	n, err := s.targetRepository.Prune(ctx, s.cfg.LinkCheck.TargetTTLDays)
	return n, apperr.Wrap(err)
}

type ChecksService struct {
	catalog            *catalogservices.CatalogServices
	cfg                *config.Config
	checker            linksports.CheckerPort
	targetRepository   *repositories.TargetRepository
	templateRepository *repositories.TemplateRepository
	projects           *ProjectsService
}
