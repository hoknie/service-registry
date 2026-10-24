package services

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/access/usecases"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/password"
	"svc-registry/pkg/pgtx"
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
	if err := accesstypes.RequireRight(p, accesstypes.SectionUsers, accesstypes.ActionCreate); err != nil {
		return accesstypes.User{}, err
	}
	if !p.IsSuperadmin && in.IsSuperadmin {
		return accesstypes.User{}, apperr.New(apperr.Forbidden)
	}
	return s.InsertUser(ctx, in)
}

func (s *UsersService) ListUsers(ctx context.Context, p accesstypes.Principal, q accesstypes.PageQuery) (accesstypes.Page[accesstypes.User], error) {
	if err := accesstypes.RequireRight(p, accesstypes.SectionUsers, accesstypes.ActionRead); err != nil {
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
	if err := accesstypes.RequireRight(p, accesstypes.SectionUsers, accesstypes.ActionRead); err != nil {
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
	if err := accesstypes.RequireRight(p, accesstypes.SectionUsers, accesstypes.ActionUpdate); err != nil {
		return accesstypes.User{}, err
	}
	if !p.IsSuperadmin && in.IsSuperadmin != nil {
		return accesstypes.User{}, apperr.New(apperr.Forbidden)
	}
	if err := s.mayChange(ctx, p, id); err != nil {
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
	user, err := s.updateUser.Run(ctx, usecases.UpdateUserInput{ID: id, Changes: changes})
	if err != nil {
		return accesstypes.User{}, apperr.Wrap(err)
	}
	if changes.Disables() {
		if _, err := s.sessionRepository.DeleteForUser(ctx, id, nil); err != nil {
			return accesstypes.User{}, apperr.Wrap(err)
		}
	}
	return user, nil
}

func (s *UsersService) ResetPassword(ctx context.Context, p accesstypes.Principal, id uuid.UUID, password string) error {
	if err := accesstypes.RequireRight(p, accesstypes.SectionUsers, accesstypes.ActionUpdate); err != nil {
		return err
	}
	if err := s.mayChange(ctx, p, id); err != nil {
		return err
	}
	if err := accesstypes.ValidatePassword(password); err != nil {
		return apperr.Wrap(err)
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return apperr.Internalf("%v", err)
	}
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		if err := s.userRepository.SetPassword(ctx, id, hash); err != nil {
			return err
		}
		if err := s.tokenRepository.RevokeForUser(ctx, id); err != nil {
			return err
		}
		return s.grantRepository.RevokeForUser(ctx, id)
	})
	if err != nil {
		return apperr.Wrap(err)
	}
	_, err = s.sessionRepository.DeleteForUser(ctx, id, nil)
	return apperr.Wrap(err)
}

func (s *UsersService) mayChange(ctx context.Context, p accesstypes.Principal, id uuid.UUID) error {
	if p.IsSuperadmin {
		return nil
	}
	if id == p.UserID {
		return apperr.New(apperr.Forbidden)
	}
	user, err := s.userRepository.Find(ctx, id)
	if err != nil || user == nil {
		return apperr.Wrap(err)
	}
	return protectedUser(ctx, s.permissionRepository, *user)
}

func protectedUser(ctx context.Context, r *repositories.GroupPermissionRepository, user accesstypes.User) error {
	if user.IsSuperadmin {
		return apperr.New(apperr.Forbidden)
	}
	privileged, err := r.UserHasAny(ctx, user.ID)
	if err != nil {
		return apperr.Wrap(err)
	}
	if privileged {
		return apperr.New(apperr.Forbidden)
	}
	return nil
}

type UsersService struct {
	db                   *pgtx.DB
	updateUser           *usecases.UpdateUserUseCase
	tokenRepository      *repositories.TokenRepository
	grantRepository      *repositories.OAuthGrantRepository
	permissionRepository *repositories.GroupPermissionRepository
	hasher               *password.Hasher
	sessionRepository    *repositories.SessionRepository
	userRepository       *repositories.UserRepository
}

const BootstrapAdminName = "Administrator"

func CheckBootstrapAdmin(admin config.BootstrapAdmin) error {
	if _, err := accesstypes.ValidateEmail(admin.Email); err != nil {
		return errors.New("BOOTSTRAP_ADMIN_EMAIL " + err.Error())
	}
	if err := accesstypes.ValidatePassword(admin.Password); err != nil {
		return errors.New("BOOTSTRAP_ADMIN_PASSWORD " + err.Error())
	}
	return nil
}

func (s *BootstrapService) PrepareBootstrapAdmin(admin config.BootstrapAdmin) (accesstypes.NewUser, error) {
	return prepareUser(s.hasher, accesstypes.CreateUser{
		Email: admin.Email, DisplayName: BootstrapAdminName, Password: admin.Password, IsSuperadmin: true,
	})
}

func (s *BootstrapService) EnsureBootstrapAdmin(ctx context.Context, admin accesstypes.NewUser) (bool, error) {
	inserted, err := s.userRepository.InsertIfNone(ctx, admin)
	if errors.Is(err, accesstypes.ConflictEmailTaken) {
		return false, nil
	}
	return inserted, err
}

type BootstrapService struct {
	hasher         *password.Hasher
	userRepository *repositories.UserRepository
}
