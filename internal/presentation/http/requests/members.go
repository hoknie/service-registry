package requests

import (
	"github.com/google/uuid"

	membershipservices "svc-registry/internal/feature/membership/services"
)

type MemberAdd struct {
	UserID *uuid.UUID `json:"user_id"`
	Email  *string    `json:"email"`
	Role   *string    `json:"role"`
}

func (r MemberAdd) Input() membershipservices.AddMember {
	return membershipservices.AddMember{UserID: r.UserID, Email: r.Email, Role: r.Role}
}

type MemberRole struct {
	Role Optional[string] `json:"role"`
}

type InvitationCreate struct {
	Email  *string `json:"email" validate:"required"`
	Role   *string `json:"role"`
	Locale *string `json:"locale"`
}

func (r InvitationCreate) Input() membershipservices.NewInvitation {
	role, locale := "viewer", "en"
	if r.Role != nil {
		role = *r.Role
	}
	if r.Locale != nil {
		locale = *r.Locale
	}
	return membershipservices.NewInvitation{Email: *r.Email, Role: role, Locale: locale}
}

type InvitationResend struct {
	Locale *string `json:"locale"`
}

type InvitationRegister struct {
	Name     *string `json:"name" validate:"required"`
	Password *string `json:"password" validate:"required"`
}
