package services

import (
	"context"

	"github.com/google/uuid"

	catalogservices "svc-registry/internal/feature/catalog/services"
	linksports "svc-registry/internal/feature/links/ports"
	"svc-registry/internal/feature/links/repositories"
	linkstypes "svc-registry/internal/feature/links/types"
	notifyservices "svc-registry/internal/feature/notify/services"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

func (s *ProjectsService) ProjectBadges(ctx context.Context, projects []uuid.UUID) (map[uuid.UUID][]linkstypes.Badge, error) {
	data, err := s.templateRepository.ProjectsData(ctx, projects)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make(map[uuid.UUID][]linkstypes.Badge, len(projects))
	for _, id := range projects {
		b := data[id]
		out[id] = linkstypes.Badges(b.Context(id), b.Templates)
	}
	return out, nil
}

type ProjectsService struct {
	catalog            *catalogservices.CatalogServices
	cfg                *config.Config
	checker            linksports.CheckerPort
	db                 *pgtx.DB
	notify             *notifyservices.NotifyServices
	targetRepository   *repositories.TargetRepository
	templateRepository *repositories.TemplateRepository
}
