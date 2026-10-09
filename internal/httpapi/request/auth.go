package request

import "svc-registry/internal/access"

type Login struct {
	Email    *string `json:"email" validate:"required"`
	Password *string `json:"password" validate:"required"`
}

func (r Login) Credentials() access.Credentials {
	return access.Credentials{Email: *r.Email, Password: *r.Password}
}

type ChangePassword struct {
	CurrentPassword *string `json:"current_password"`
	NewPassword     *string `json:"new_password" validate:"required"`
}

func (r ChangePassword) Change() access.PasswordChange {
	return access.PasswordChange{CurrentPassword: r.CurrentPassword, NewPassword: *r.NewPassword}
}
