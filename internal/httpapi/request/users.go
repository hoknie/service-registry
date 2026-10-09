package request

import "svc-registry/internal/access"

type CreateUser struct {
	Email        *string `json:"email" validate:"required"`
	DisplayName  *string `json:"display_name" validate:"required"`
	Password     *string `json:"password" validate:"required"`
	IsSuperadmin bool    `json:"is_superadmin"`
}

func (r CreateUser) User() access.CreateUser {
	return access.CreateUser{Email: *r.Email, DisplayName: *r.DisplayName, Password: *r.Password, IsSuperadmin: r.IsSuperadmin}
}

type UpdateUser struct {
	DisplayName  *string `json:"display_name"`
	Status       *string `json:"status"`
	IsSuperadmin *bool   `json:"is_superadmin"`
}

func (r UpdateUser) Changes() access.UpdateUser {
	return access.UpdateUser{DisplayName: r.DisplayName, Status: r.Status, IsSuperadmin: r.IsSuperadmin}
}

type ResetPassword struct {
	Password *string `json:"password" validate:"required"`
}
