package services

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/apperr"
)

type SpacesService struct{ parts }

type Space struct {
	Organization gatewaytypes.Ref
	Description  string
	CanWrite     bool
	CanManage    bool
}

func (s *SpacesService) organization(ctx context.Context, p accesstypes.Principal, org uuid.UUID) (gatewaytypes.Ref, error) {
	sc, err := s.catalog.Access.Scope(ctx, p, org)
	if err != nil {
		return gatewaytypes.Ref{}, err
	}
	if len(sc.Path) > 0 {
		return gatewaytypes.Ref{}, apperr.New(apperr.NotFound)
	}
	return gatewaytypes.Ref{ID: sc.Node.ID, Slug: sc.Node.Slug, Name: sc.Node.Name}, nil
}

func (s *SpacesService) OrganizationSpace(ctx context.Context, p accesstypes.Principal, org uuid.UUID) (Space, error) {
	ref, err := s.organization(ctx, p, org)
	if err != nil {
		return Space{}, err
	}
	row, err := s.organizations.Get(ctx, org)
	if err != nil {
		return Space{}, apperr.Wrap(err)
	}
	scope := gatewaytypes.Scope{OrganizationID: &org}
	return Space{Organization: ref, Description: row.Description, CanWrite: s.access.writes(ctx, p, scope),
		CanManage: s.access.manages(ctx, p, scope)}, nil
}

func (s *SpacesService) PutOrganizationSpace(ctx context.Context, p accesstypes.Principal, org uuid.UUID, description string) (Space, error) {
	if _, err := s.organization(ctx, p, org); err != nil {
		return Space{}, err
	}
	if _, err := s.catalog.Access.Authorize(ctx, p, catalogtypes.PermWrite, org); err != nil {
		return Space{}, err
	}
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > 2000 {
		return Space{}, apperr.Wrap(gatewaytypes.InvalidSpace)
	}
	if err := s.organizations.SetDescription(ctx, org, description); err != nil {
		return Space{}, apperr.Wrap(err)
	}
	return s.OrganizationSpace(ctx, p, org)
}
