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
	out := make(map[uuid.UUID][]linkstypes.Badge, len(projects))
	if len(projects) == 0 {
		return out, nil
	}
	var data map[uuid.UUID]linkstypes.ProjectBatch
	err := s.db.InTxOpts(ctx, snapshot, func(ctx context.Context) error {
		var err error
		data, err = s.templateRepository.ProjectsData(ctx, projects)
		return err
	})
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	for _, id := range projects {
		b := data[id]
		linkstypes.SortTemplates(b.Templates)
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
