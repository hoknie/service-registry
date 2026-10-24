package usecases

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
)

type UpdateUserInput struct {
	ID      uuid.UUID
	Changes accesstypes.UserChanges
}

type UpdateUserUseCase struct {
	db                *pgtx.DB
	userRepository    *repositories.UserRepository
	sessionRepository *repositories.SessionRepository
	tokenRepository   *repositories.TokenRepository
	grantRepository   *repositories.OAuthGrantRepository
}

type UpdateUserDeps struct {
	DB       *pgtx.DB
	Users    *repositories.UserRepository
	Sessions *repositories.SessionRepository
	Tokens   *repositories.TokenRepository
	Grants   *repositories.OAuthGrantRepository
}

func NewUpdateUser(d UpdateUserDeps) *UpdateUserUseCase {
	return &UpdateUserUseCase{db: d.DB, userRepository: d.Users, sessionRepository: d.Sessions, tokenRepository: d.Tokens,
		grantRepository: d.Grants}
}

func (u *UpdateUserUseCase) Run(ctx context.Context, in UpdateUserInput) (accesstypes.User, error) {
	var user accesstypes.User
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		if err := u.lock(ctx, in); err != nil {
			return err
		}
		var err error
		if user, err = u.userRepository.Update(ctx, in.ID, in.Changes); err != nil {
			return err
		}
		return u.revoke(ctx, in)
	})
	return user, err
}

func (u *UpdateUserUseCase) lock(ctx context.Context, in UpdateUserInput) error {
	if in.Changes.Demotes() || in.Changes.Disables() {
		admins, err := u.userRepository.LockActiveSuperadmins(ctx)
		if err != nil {
			return err
		}
		if accesstypes.LastSuperadmin(admins, in.ID) {
			return accesstypes.ConflictLastSuperadmin
		}
	}
	found, err := u.userRepository.Lock(ctx, in.ID)
	if err != nil {
		return err
	}
	if !found {
		return apperr.ErrNotFound
	}
	return nil
}

func (u *UpdateUserUseCase) revoke(ctx context.Context, in UpdateUserInput) error {
	if in.Changes.MakesService() {
		if _, err := u.sessionRepository.DeleteForUser(ctx, in.ID, nil); err != nil {
			return err
		}
	}
	switch {
	case in.Changes.Disables():
		if err := u.tokenRepository.RevokeForUser(ctx, in.ID); err != nil {
			return err
		}
		return u.grantRepository.RevokeForUser(ctx, in.ID)
	case in.Changes.Demotes():
		return u.tokenRepository.RevokeUnexpiring(ctx, in.ID)
	}
	return nil
}
