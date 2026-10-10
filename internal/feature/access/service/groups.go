package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/apperr"
)

func (s *Service) CreateGroup(ctx context.Context, p access.Principal, in access.GroupName) (access.Group, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return access.Group{}, err
	}
	name, err := access.ValidateGroupName(in.Name)
	if err != nil {
		return access.Group{}, apperr.Wrap(err)
	}
	g, err := s.groups.Insert(ctx, access.NewGroup{ID: uuid.Must(uuid.NewV7()), Name: name})
	return g, apperr.Wrap(err)
}

func (s *Service) ListGroups(ctx context.Context, p access.Principal, q access.PageQuery) (access.Page[access.Group], error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return access.Page[access.Group]{}, err
	}
	page, err := access.ValidatePage(q)
	if err != nil {
		return access.Page[access.Group]{}, apperr.Wrap(err)
	}
	out, err := s.groups.List(ctx, page)
	return out, apperr.Wrap(err)
}

func (s *Service) UserGroups(ctx context.Context, p access.Principal, userID uuid.UUID) ([]access.Group, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return nil, err
	}
	user, err := s.users.Find(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if user == nil {
		return nil, apperr.New(apperr.NotFound)
	}
	groups, err := s.groups.ForUser(ctx, userID)
	return groups, apperr.Wrap(err)
}

func (s *Service) GetGroup(ctx context.Context, p access.Principal, id uuid.UUID) (access.GroupDetails, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return access.GroupDetails{}, err
	}
	g, err := s.groups.Find(ctx, id)
	if err != nil {
		return access.GroupDetails{}, apperr.Wrap(err)
	}
	if g == nil {
		return access.GroupDetails{}, apperr.New(apperr.NotFound)
	}
	return *g, nil
}

func (s *Service) RenameGroup(ctx context.Context, p access.Principal, id uuid.UUID, in access.GroupName) (access.Group, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return access.Group{}, err
	}
	name, err := access.ValidateGroupName(in.Name)
	if err != nil {
		return access.Group{}, apperr.Wrap(err)
	}
	g, err := s.groups.Rename(ctx, id, name)
	return g, apperr.Wrap(err)
}

func (s *Service) DeleteGroup(ctx context.Context, p access.Principal, id uuid.UUID) error {
	if err := access.RequireSuperadmin(p); err != nil {
		return err
	}
	return apperr.Wrap(s.groups.Delete(ctx, id))
}

func (s *Service) AddMember(ctx context.Context, p access.Principal, groupID, userID uuid.UUID) error {
	if err := access.RequireSuperadmin(p); err != nil {
		return err
	}
	return apperr.Wrap(s.groups.AddMember(ctx, uuid.Must(uuid.NewV7()), groupID, userID))
}

func (s *Service) RemoveMember(ctx context.Context, p access.Principal, groupID, userID uuid.UUID) error {
	if err := access.RequireSuperadmin(p); err != nil {
		return err
	}
	return apperr.Wrap(s.groups.RemoveMember(ctx, groupID, userID))
}
