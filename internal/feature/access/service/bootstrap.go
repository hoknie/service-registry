package service

import (
	"context"
	"errors"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/config"
)

const BootstrapAdminName = "Administrator"

func CheckBootstrapAdmin(admin config.BootstrapAdmin) error {
	if _, err := access.ValidateEmail(admin.Email); err != nil {
		return errors.New("BOOTSTRAP_ADMIN_EMAIL " + err.Error())
	}
	if err := access.ValidatePassword(admin.Password); err != nil {
		return errors.New("BOOTSTRAP_ADMIN_PASSWORD " + err.Error())
	}
	return nil
}

func (s *Service) PrepareBootstrapAdmin(admin config.BootstrapAdmin) (access.NewUser, error) {
	return prepareUser(s.hasher, access.CreateUser{
		Email: admin.Email, DisplayName: BootstrapAdminName, Password: admin.Password, IsSuperadmin: true,
	})
}

func (s *Service) EnsureBootstrapAdmin(ctx context.Context, admin access.NewUser) (bool, error) {
	return s.users.InsertIfNone(ctx, admin)
}
