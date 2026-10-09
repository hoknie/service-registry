package request

import "svc-registry/internal/access"

type CreateUser struct {
	Email        *string `json:"email" validate:"required"`
	DisplayName  *string `json:"display_name" validate:"required"`
	Password     *string `json:"password" validate:"required_unless=IsService true"`
	IsSuperadmin bool    `json:"is_superadmin"`
	IsService    bool    `json:"is_service"`
}

func (r CreateUser) User() access.CreateUser {
	password := ""
	if r.Password != nil {
		password = *r.Password
	}
	return access.CreateUser{Email: *r.Email, DisplayName: *r.DisplayName, Password: password, IsSuperadmin: r.IsSuperadmin,
		IsService: r.IsService}
}

type UpdateUser struct {
	DisplayName  *string `json:"display_name"`
	Status       *string `json:"status"`
	IsSuperadmin *bool   `json:"is_superadmin"`
	IsService    *bool   `json:"is_service"`
}

func (r UpdateUser) Changes() access.UpdateUser {
	return access.UpdateUser{DisplayName: r.DisplayName, Status: r.Status, IsSuperadmin: r.IsSuperadmin, IsService: r.IsService}
}

type ResetPassword struct {
	Password *string `json:"password" validate:"required"`
}
