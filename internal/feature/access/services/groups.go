package services

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
)

func (s *GroupsService) CreateGroup(ctx context.Context, p accesstypes.Principal, in accesstypes.GroupName) (accesstypes.Group, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return accesstypes.Group{}, err
	}
	name, err := accesstypes.ValidateGroupName(in.Name)
	if err != nil {
		return accesstypes.Group{}, apperr.Wrap(err)
	}
	g, err := s.groupRepository.Insert(ctx, accesstypes.NewGroup{ID: uuid.Must(uuid.NewV7()), Name: name})
	return g, apperr.Wrap(err)
}

func (s *GroupsService) ListGroups(ctx context.Context, p accesstypes.Principal, q accesstypes.PageQuery) (accesstypes.Page[accesstypes.Group], error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return accesstypes.Page[accesstypes.Group]{}, err
	}
	page, err := accesstypes.ValidatePage(q)
	if err != nil {
		return accesstypes.Page[accesstypes.Group]{}, apperr.Wrap(err)
	}
	out, err := s.groupRepository.List(ctx, page)
	return out, apperr.Wrap(err)
}

func (s *GroupsService) UserGroups(ctx context.Context, p accesstypes.Principal, userID uuid.UUID) ([]accesstypes.Group, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return nil, err
	}
	user, err := s.userRepository.Find(ctx, userID)
	if err != nil {
		return nil, apperr.Wrap(err)
	}
	if user == nil {
		return nil, apperr.New(apperr.NotFound)
	}
	groups, err := s.groupRepository.ForUser(ctx, userID)
	return groups, apperr.Wrap(err)
}

func (s *GroupsService) GetGroup(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (accesstypes.GroupDetails, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return accesstypes.GroupDetails{}, err
	}
	g, err := s.groupRepository.Find(ctx, id)
	if err != nil {
		return accesstypes.GroupDetails{}, apperr.Wrap(err)
	}
	if g == nil {
		return accesstypes.GroupDetails{}, apperr.New(apperr.NotFound)
	}
	return *g, nil
}

func (s *GroupsService) RenameGroup(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in accesstypes.GroupName) (accesstypes.Group, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return accesstypes.Group{}, err
	}
	name, err := accesstypes.ValidateGroupName(in.Name)
	if err != nil {
		return accesstypes.Group{}, apperr.Wrap(err)
	}
	g, err := s.groupRepository.Rename(ctx, id, name)
	return g, apperr.Wrap(err)
}

func (s *GroupsService) DeleteGroup(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return err
	}
	return apperr.Wrap(s.groupRepository.Delete(ctx, id))
}

func (s *GroupsService) AddMember(ctx context.Context, p accesstypes.Principal, groupID, userID uuid.UUID) error {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return err
	}
	return apperr.Wrap(s.groupRepository.AddMember(ctx, uuid.Must(uuid.NewV7()), groupID, userID))
}

func (s *GroupsService) RemoveMember(ctx context.Context, p accesstypes.Principal, groupID, userID uuid.UUID) error {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return err
	}
	return apperr.Wrap(s.groupRepository.RemoveMember(ctx, groupID, userID))
}

type GroupsService struct {
	groupRepository *repositories.GroupRepository
	userRepository  *repositories.UserRepository
}
