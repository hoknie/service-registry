package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/apperr"
	"svc-registry/internal/auth"
)

func NewUser(hasher *auth.PasswordHasher, in access.CreateUser) (access.NewUser, error) {
	email, err := access.ValidateEmail(in.Email)
	if err != nil {
		return access.NewUser{}, apperr.Wrap(err)
	}
	name, err := access.ValidateDisplayName(in.DisplayName)
	if err != nil {
		return access.NewUser{}, apperr.Wrap(err)
	}
	out := access.NewUser{ID: uuid.Must(uuid.NewV7()), Email: email, DisplayName: name, IsSuperadmin: in.IsSuperadmin,
		IsService: in.IsService}
	if in.IsService && in.Password == "" {
		return out, nil
	}
	if err := access.ValidatePassword(in.Password); err != nil {
		return access.NewUser{}, apperr.Wrap(err)
	}
	hash, err := hasher.Hash(in.Password)
	if err != nil {
		return access.NewUser{}, apperr.Internalf("%v", err)
	}
	out.PasswordHash = hash
	return out, nil
}

func CreateUserWith(ctx context.Context, users access.UserStore, hasher *auth.PasswordHasher, in access.CreateUser) (access.User, error) {
	row, err := NewUser(hasher, in)
	if err != nil {
		return access.User{}, err
	}
	user, err := users.Insert(ctx, row)
	return user, apperr.Wrap(err)
}

func CreateUser(ctx context.Context, state *State, p Principal, in access.CreateUser) (access.User, error) {
	if err := RequireSuperadmin(p); err != nil {
		return access.User{}, err
	}
	return CreateUserWith(ctx, state.Users, state.Hasher, in)
}

func ListUsers(ctx context.Context, state *State, p Principal, q access.PageQuery) (access.Page[access.User], error) {
	if err := RequireSuperadmin(p); err != nil {
		return access.Page[access.User]{}, err
	}
	page, err := access.ValidatePage(q)
	if err != nil {
		return access.Page[access.User]{}, apperr.Wrap(err)
	}
	out, err := state.Users.List(ctx, page)
	return out, apperr.Wrap(err)
}

func GetUser(ctx context.Context, state *State, p Principal, id uuid.UUID) (access.User, error) {
	if err := RequireSuperadmin(p); err != nil {
		return access.User{}, err
	}
	user, err := state.Users.Find(ctx, id)
	if err != nil {
		return access.User{}, apperr.Wrap(err)
	}
	if user == nil {
		return access.User{}, apperr.New(apperr.NotFound)
	}
	return *user, nil
}

func UpdateUser(ctx context.Context, state *State, p Principal, id uuid.UUID, in access.UpdateUser) (access.User, error) {
	if err := RequireSuperadmin(p); err != nil {
		return access.User{}, err
	}
	var changes access.UserChanges
	if in.DisplayName != nil {
		name, err := access.ValidateDisplayName(*in.DisplayName)
		if err != nil {
			return access.User{}, apperr.Wrap(err)
		}
		changes.DisplayName = &name
	}
	if in.Status != nil {
		st, err := access.ValidateStatus(*in.Status)
		if err != nil {
			return access.User{}, apperr.Wrap(err)
		}
		changes.Status = &st
	}
	changes.IsSuperadmin = in.IsSuperadmin
	changes.IsService = in.IsService
	if in.IsService != nil && *in.IsService && id == p.UserID {
		return access.User{}, apperr.Wrap(access.ConflictSelfService)
	}
	user, err := state.Users.Update(ctx, id, changes)
	if err != nil {
		return access.User{}, apperr.Wrap(err)
	}
	if changes.Status != nil && *changes.Status == access.StatusDisabled {
		if _, err := state.Sessions.DeleteForUser(ctx, id, nil); err != nil {
			return access.User{}, apperr.Wrap(err)
		}
	}
	return user, nil
}

func ResetPassword(ctx context.Context, state *State, p Principal, id uuid.UUID, password string) error {
	if err := RequireSuperadmin(p); err != nil {
		return err
	}
	if err := access.ValidatePassword(password); err != nil {
		return apperr.Wrap(err)
	}
	hash, err := state.Hasher.Hash(password)
	if err != nil {
		return apperr.Internalf("%v", err)
	}
	if err := state.Users.ResetPassword(ctx, id, hash); err != nil {
		return apperr.Wrap(err)
	}
	_, err = state.Sessions.DeleteForUser(ctx, id, nil)
	return apperr.Wrap(err)
}
