package services

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/feature/skills/repositories"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/platform/apperr"
)

type SpacesService struct {
	parts
	spaces *repositories.SpaceRepository
}

type Space struct {
	Organization skillstypes.Ref
	Description  string
	CanWrite     bool
}

func (s *SpacesService) OrganizationSpace(ctx context.Context, p accesstypes.Principal, org uuid.UUID) (Space, error) {
	sc, err := s.access.catalog.Access.Scope(ctx, p, org)
	if err != nil {
		return Space{}, err
	}
	if len(sc.Path) > 0 {
		return Space{}, apperr.New(apperr.NotFound)
	}
	description, err := s.spaces.Description(ctx, org)
	if err != nil {
		return Space{}, apperr.Wrap(err)
	}
	return Space{Organization: skillstypes.Ref{ID: sc.Node.ID, Slug: sc.Node.Slug, Name: sc.Node.Name}, Description: description,
		CanWrite: p.IsSuperadmin || sc.Allows(catalogtypes.PermWrite)}, nil
}

func (s *SpacesService) PutOrganizationSpace(ctx context.Context, p accesstypes.Principal, org uuid.UUID, description string) (Space, error) {
	sc, err := s.access.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, org)
	if err != nil {
		return Space{}, err
	}
	if len(sc.Path) > 0 {
		return Space{}, apperr.New(apperr.NotFound)
	}
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > 2000 {
		return Space{}, apperr.Wrap(skillstypes.InvalidSpace)
	}
	if err := s.spaces.SetDescription(ctx, org, description); err != nil {
		return Space{}, apperr.Wrap(err)
	}
	return s.OrganizationSpace(ctx, p, org)
}
