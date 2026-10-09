package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
)

func CreateGroup(ctx context.Context, state *State, p Principal, in access.GroupName) (access.Group, error) {
	if err := RequireSuperadmin(p); err != nil {
		return access.Group{}, err
	}
	name, err := access.ValidateGroupName(in.Name)
	if err != nil {
		return access.Group{}, apperr.Wrap(err)
	}
	g, err := state.Groups.Insert(ctx, access.NewGroup{ID: uuid.Must(uuid.NewV7()), Name: name})
	return g, apperr.Wrap(err)
}

func ListGroups(ctx context.Context, state *State, p Principal, q access.PageQuery) (access.Page[access.Group], error) {
	if err := RequireSuperadmin(p); err != nil {
		return access.Page[access.Group]{}, err
	}
	page, err := access.ValidatePage(q)
	if err != nil {
		return access.Page[access.Group]{}, apperr.Wrap(err)
	}
	out, err := state.Groups.List(ctx, page)
	return out, apperr.Wrap(err)
}

func GetGroup(ctx context.Context, state *State, p Principal, id uuid.UUID) (access.GroupDetails, error) {
	if err := RequireSuperadmin(p); err != nil {
		return access.GroupDetails{}, err
	}
	g, err := state.Groups.Find(ctx, id)
	if err != nil {
		return access.GroupDetails{}, apperr.Wrap(err)
	}
	if g == nil {
		return access.GroupDetails{}, apperr.New(apperr.NotFound)
	}
	return *g, nil
}

func RenameGroup(ctx context.Context, state *State, p Principal, id uuid.UUID, in access.GroupName) (access.Group, error) {
	if err := RequireSuperadmin(p); err != nil {
		return access.Group{}, err
	}
	name, err := access.ValidateGroupName(in.Name)
	if err != nil {
		return access.Group{}, apperr.Wrap(err)
	}
	g, err := state.Groups.Rename(ctx, id, name)
	return g, apperr.Wrap(err)
}

func DeleteGroup(ctx context.Context, state *State, p Principal, id uuid.UUID) error {
	if err := RequireSuperadmin(p); err != nil {
		return err
	}
	return apperr.Wrap(state.Groups.Delete(ctx, id))
}

func AddMember(ctx context.Context, state *State, p Principal, groupID, userID uuid.UUID) error {
	if err := RequireSuperadmin(p); err != nil {
		return err
	}
	return apperr.Wrap(state.Groups.AddMember(ctx, uuid.Must(uuid.NewV7()), groupID, userID))
}

func RemoveMember(ctx context.Context, state *State, p Principal, groupID, userID uuid.UUID) error {
	if err := RequireSuperadmin(p); err != nil {
		return err
	}
	return apperr.Wrap(state.Groups.RemoveMember(ctx, groupID, userID))
}
