package services

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/password"
)

func prepareUser(hasher *password.Hasher, in accesstypes.CreateUser) (accesstypes.NewUser, error) {
	email, err := accesstypes.ValidateEmail(in.Email)
	if err != nil {
		return accesstypes.NewUser{}, apperr.Wrap(err)
	}
	name, err := accesstypes.ValidateDisplayName(in.DisplayName)
	if err != nil {
		return accesstypes.NewUser{}, apperr.Wrap(err)
	}
	out := accesstypes.NewUser{ID: uuid.Must(uuid.NewV7()), Email: email, DisplayName: name, IsSuperadmin: in.IsSuperadmin,
		IsService: in.IsService}
	if in.IsService && in.Password == "" {
		return out, nil
	}
	if err := accesstypes.ValidatePassword(in.Password); err != nil {
		return accesstypes.NewUser{}, apperr.Wrap(err)
	}
	hash, err := hasher.Hash(in.Password)
	if err != nil {
		return accesstypes.NewUser{}, apperr.Internalf("%v", err)
	}
	out.PasswordHash = hash
	return out, nil
}

func (s *UsersService) InsertUser(ctx context.Context, in accesstypes.CreateUser) (accesstypes.User, error) {
	row, err := prepareUser(s.hasher, in)
	if err != nil {
		return accesstypes.User{}, err
	}
	user, err := s.userRepository.Insert(ctx, row)
	return user, apperr.Wrap(err)
}

func (s *UsersService) CreateUser(ctx context.Context, p accesstypes.Principal, in accesstypes.CreateUser) (accesstypes.User, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return accesstypes.User{}, err
	}
	return s.InsertUser(ctx, in)
}

func (s *UsersService) ListUsers(ctx context.Context, p accesstypes.Principal, q accesstypes.PageQuery) (accesstypes.Page[accesstypes.User], error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return accesstypes.Page[accesstypes.User]{}, err
	}
	page, err := accesstypes.ValidatePage(q)
	if err != nil {
		return accesstypes.Page[accesstypes.User]{}, apperr.Wrap(err)
	}
	out, err := s.userRepository.List(ctx, page)
	return out, apperr.Wrap(err)
}

func (s *UsersService) GetUser(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (accesstypes.User, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return accesstypes.User{}, err
	}
	user, err := s.userRepository.Find(ctx, id)
	if err != nil {
		return accesstypes.User{}, apperr.Wrap(err)
	}
	if user == nil {
		return accesstypes.User{}, apperr.New(apperr.NotFound)
	}
	return *user, nil
}

func (s *UsersService) UpdateUser(ctx context.Context, p accesstypes.Principal, id uuid.UUID, in accesstypes.UpdateUser) (accesstypes.User, error) {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return accesstypes.User{}, err
	}
	var changes accesstypes.UserChanges
	if in.DisplayName != nil {
		name, err := accesstypes.ValidateDisplayName(*in.DisplayName)
		if err != nil {
			return accesstypes.User{}, apperr.Wrap(err)
		}
		changes.DisplayName = &name
	}
	if in.Status != nil {
		st, err := accesstypes.ValidateStatus(*in.Status)
		if err != nil {
			return accesstypes.User{}, apperr.Wrap(err)
		}
		changes.Status = &st
	}
	changes.IsSuperadmin = in.IsSuperadmin
	changes.IsService = in.IsService
	if in.IsService != nil && *in.IsService && id == p.UserID {
		return accesstypes.User{}, apperr.Wrap(accesstypes.ConflictSelfService)
	}
	user, err := s.userRepository.Update(ctx, id, changes)
	if err != nil {
		return accesstypes.User{}, apperr.Wrap(err)
	}
	if changes.Status != nil && *changes.Status == accesstypes.StatusDisabled {
		if _, err := s.sessionRepository.DeleteForUser(ctx, id, nil); err != nil {
			return accesstypes.User{}, apperr.Wrap(err)
		}
	}
	return user, nil
}

func (s *UsersService) ResetPassword(ctx context.Context, p accesstypes.Principal, id uuid.UUID, password string) error {
	if err := accesstypes.RequireSuperadmin(p); err != nil {
		return err
	}
	if err := accesstypes.ValidatePassword(password); err != nil {
		return apperr.Wrap(err)
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return apperr.Internalf("%v", err)
	}
	if err := s.userRepository.ResetPassword(ctx, id, hash); err != nil {
		return apperr.Wrap(err)
	}
	_, err = s.sessionRepository.DeleteForUser(ctx, id, nil)
	return apperr.Wrap(err)
}

type UsersService struct {
	hasher            *password.Hasher
	sessionRepository *repositories.SessionRepository
	userRepository    *repositories.UserRepository
}
