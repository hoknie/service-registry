package service

import (
	"context"
	"errors"

	"svc-registry/internal/access"
	"svc-registry/internal/auth"
	"svc-registry/internal/config"
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

func PrepareBootstrapAdmin(hasher *auth.PasswordHasher, admin config.BootstrapAdmin) (access.NewUser, error) {
	return NewUser(hasher, access.CreateUser{
		Email: admin.Email, DisplayName: BootstrapAdminName, Password: admin.Password, IsSuperadmin: true,
	})
}

func EnsureBootstrapAdmin(ctx context.Context, state *State, admin access.NewUser) (bool, error) {
	return state.Users.InsertIfNone(ctx, admin)
}
