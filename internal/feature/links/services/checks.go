package services

import (
	"context"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
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
	_, err := s.projects.record(ctx, t.URL, r)
	return err
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
			links, err := s.projects.systemLinks(ctx, id)
			if err != nil {
				return seen, err
			}
			if err := s.projects.seen(ctx, id, links); err != nil {
				return seen, apperr.Wrap(err)
			}
			seen += len(linkstypes.URLs(links))
		}
		if len(ids) < projectPageSize {
			return seen, nil
		}
		after = ids[len(ids)-1]
	}
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
