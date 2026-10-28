package services

import (
	"context"
	"strings"
	"unicode"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
)

func (s *BindingsService) ListBindings(ctx context.Context, p accesstypes.Principal, id uuid.UUID) ([]catalogtypes.Binding, error) {
	scoped, err := s.access.Authorize(ctx, p, catalogtypes.PermAccess, id)
	if err != nil {
		return nil, err
	}
	ids := []uuid.UUID{scoped.Node.ID}
	for i := len(scoped.Path) - 1; i >= 0; i-- {
		ids = append(ids, scoped.Path[i].Node.ID)
	}
	out, err := s.bindingRepository.List(ctx, ids)
	return out, apperr.Wrap(err)
}

func (s *BindingsService) GrantRole(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in catalogtypes.GrantRole) (catalogtypes.Binding, error) {
	if _, err := s.access.Authorize(ctx, p, catalogtypes.PermAccess, id); err != nil {
		return catalogtypes.Binding{}, err
	}
	kind, ok := catalogtypes.ParseSubjectKind(in.SubjectKind)
	if !ok {
		return catalogtypes.Binding{}, apperr.Wrap(catalogtypes.InvalidSubjectKind)
	}
	role, ok := catalogtypes.ParseRole(in.Role)
	if !ok {
		return catalogtypes.Binding{}, apperr.Wrap(catalogtypes.InvalidRole)
	}
	b, err := s.bindingRepository.Put(ctx, catalogtypes.NewBinding{
		ID: uuid.Must(uuid.NewV7()), NodeID: id, SubjectKind: kind,
		Subject: strings.TrimFunc(in.Subject, unicode.IsSpace), Role: role,
	})
	if err != nil {
		return catalogtypes.Binding{}, apperr.Wrap(err)
	}
	if b == nil {
		return catalogtypes.Binding{}, apperr.Wrap(catalogtypes.InvalidUnknownSubject)
	}
	return *b, nil
}

func (s *BindingsService) RevokeBinding(ctx context.Context, p accesstypes.Principal, id, bindingID uuid.UUID) error {
	if _, err := s.access.Authorize(ctx, p, catalogtypes.PermAccess, id); err != nil {
		return err
	}
	return apperr.Wrap(s.bindingRepository.Delete(ctx, id, bindingID))
}

type BindingsService struct {
	bindingRepository *repositories.BindingRepository
	access            *AccessService
}

func (s *BindingsService) GroupNodes(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	out, err := s.bindingRepository.GroupNodes(ctx, groupIDs)
	return out, apperr.Wrap(err)
}

type OrganizationRolesService struct {
	bindingRepository *repositories.BindingRepository
}

func (s *OrganizationRolesService) Roles(ctx context.Context, org uuid.UUID, users []uuid.UUID) (map[uuid.UUID]string, error) {
	out, err := s.bindingRepository.UserRoles(ctx, org, users)
	return out, apperr.Wrap(err)
}

func (s *OrganizationRolesService) SetRole(ctx context.Context, org, user uuid.UUID, role *string) error {
	if role == nil {
		return apperr.Wrap(s.bindingRepository.RevokeUser(ctx, org, user))
	}
	r, ok := catalogtypes.ParseRole(*role)
	if !ok {
		return apperr.UnknownValue("role", *role)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return apperr.Wrap(err)
	}
	return apperr.Wrap(s.bindingRepository.GrantUser(ctx, id, org, user, r))
}

func (s *OrganizationRolesService) GlobalGroups(ctx context.Context, org uuid.UUID) ([]catalogtypes.GroupAccess, error) {
	out, err := s.bindingRepository.GlobalGroupsIn(ctx, org)
	return out, apperr.Wrap(err)
}
