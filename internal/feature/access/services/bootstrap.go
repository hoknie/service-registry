package services

import (
	"context"
	"errors"

	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
)

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
	return s.userRepository.InsertIfNone(ctx, admin)
}

type BootstrapService struct {
	hasher         *auth.PasswordHasher
	userRepository *repositories.UserRepository
}
