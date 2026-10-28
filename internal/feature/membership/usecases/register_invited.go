package usecases

import (
	"context"

	accessservices "svc-registry/internal/feature/access/services"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/feature/membership/repositories"
	membershiptypes "svc-registry/internal/feature/membership/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
)

type RegisterInvitedDeps struct {
	DB          *pgtx.DB
	Members     *repositories.MemberRepository
	Invitations *repositories.InvitationRepository
	Access      *accessservices.AccessServices
	Accept      *AcceptInvitationUseCase
}

type RegisterInvitedUseCase struct{ d RegisterInvitedDeps }

func NewRegisterInvited(d RegisterInvitedDeps) *RegisterInvitedUseCase {
	return &RegisterInvitedUseCase{d: d}
}

type Registration struct {
	Hash     [32]byte
	Name     string
	Password string
	Previous *string
}

func (u *RegisterInvitedUseCase) Run(ctx context.Context, in Registration) (accessservices.SignedIn, error) {
	var out accessservices.SignedIn
	err := u.d.DB.InTx(ctx, func(ctx context.Context) error {
		inv, err := u.d.Invitations.ByHash(ctx, in.Hash)
		if err != nil {
			return err
		}
		if inv == nil {
			return apperr.ErrNotFound
		}
		if inv.State() != membershiptypes.Pending {
			return membershiptypes.NotPending
		}
		existing, err := u.d.Members.UserByEmail(ctx, inv.Email)
		if err != nil {
			return err
		}
		if existing != nil {
			return membershiptypes.SignInRequired
		}
		user, err := u.d.Access.Users.InsertUser(ctx, accesstypes.CreateUser{Email: inv.Email, DisplayName: in.Name, Password: in.Password})
		if err != nil {
			return err
		}
		if _, err := u.d.Accept.Run(ctx, in.Hash, Acceptor{ID: user.ID, Email: user.Email}); err != nil {
			return err
		}
		out, err = u.d.Access.Auth.SignIn(ctx, user, "password", in.Previous)
		return err
	})
	return out, err
}
