package requests

import accesstypes "svc-registry/internal/feature/access/types"

type Login struct {
	Email    *string `json:"email" validate:"required"`
	Password *string `json:"password" validate:"required"`
}

func (r Login) Credentials() accesstypes.Credentials {
	return accesstypes.Credentials{Email: *r.Email, Password: *r.Password}
}

type ChangePassword struct {
	CurrentPassword *string `json:"current_password"`
	NewPassword     *string `json:"new_password" validate:"required"`
}

func (r ChangePassword) Change() accesstypes.PasswordChange {
	return accesstypes.PasswordChange{CurrentPassword: r.CurrentPassword, NewPassword: *r.NewPassword}
}
