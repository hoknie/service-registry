package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/links"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) ProjectBadges(ctx context.Context, projects []uuid.UUID) (map[uuid.UUID][]links.Badge, error) {
	data, err := s.templates.ProjectsData(ctx, projects)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	out := make(map[uuid.UUID][]links.Badge, len(projects))
	for _, id := range projects {
		b := data[id]
		out[id] = links.Badges(b.Context(id), b.Templates)
	}
	return out, nil
}
