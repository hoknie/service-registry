package service

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
)

func prepareUser(hasher *auth.PasswordHasher, in access.CreateUser) (access.NewUser, error) {
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

func (s *Service) InsertUser(ctx context.Context, in access.CreateUser) (access.User, error) {
	row, err := prepareUser(s.hasher, in)
	if err != nil {
		return access.User{}, err
	}
	user, err := s.users.Insert(ctx, row)
	return user, apperr.Wrap(err)
}

func (s *Service) CreateUser(ctx context.Context, p access.Principal, in access.CreateUser) (access.User, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return access.User{}, err
	}
	return s.InsertUser(ctx, in)
}

func (s *Service) ListUsers(ctx context.Context, p access.Principal, q access.PageQuery) (access.Page[access.User], error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return access.Page[access.User]{}, err
	}
	page, err := access.ValidatePage(q)
	if err != nil {
		return access.Page[access.User]{}, apperr.Wrap(err)
	}
	out, err := s.users.List(ctx, page)
	return out, apperr.Wrap(err)
}

func (s *Service) GetUser(ctx context.Context, p access.Principal, id uuid.UUID) (access.User, error) {
	if err := access.RequireSuperadmin(p); err != nil {
		return access.User{}, err
	}
	user, err := s.users.Find(ctx, id)
	if err != nil {
		return access.User{}, apperr.Wrap(err)
	}
	if user == nil {
		return access.User{}, apperr.New(apperr.NotFound)
	}
	return *user, nil
}

func (s *Service) UpdateUser(ctx context.Context, p access.Principal, id uuid.UUID, in access.UpdateUser) (access.User, error) {
	if err := access.RequireSuperadmin(p); err != nil {
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
	user, err := s.users.Update(ctx, id, changes)
	if err != nil {
		return access.User{}, apperr.Wrap(err)
	}
	if changes.Status != nil && *changes.Status == access.StatusDisabled {
		if _, err := s.sessions.DeleteForUser(ctx, id, nil); err != nil {
			return access.User{}, apperr.Wrap(err)
		}
	}
	return user, nil
}

func (s *Service) ResetPassword(ctx context.Context, p access.Principal, id uuid.UUID, password string) error {
	if err := access.RequireSuperadmin(p); err != nil {
		return err
	}
	if err := access.ValidatePassword(password); err != nil {
		return apperr.Wrap(err)
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return apperr.Internalf("%v", err)
	}
	if err := s.users.ResetPassword(ctx, id, hash); err != nil {
		return apperr.Wrap(err)
	}
	_, err = s.sessions.DeleteForUser(ctx, id, nil)
	return apperr.Wrap(err)
}
