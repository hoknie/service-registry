package usecases

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"svc-registry/internal/feature/membership/repositories"
	membershiptypes "svc-registry/internal/feature/membership/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
)

type OrganizationRoles interface {
	Roles(ctx context.Context, org uuid.UUID, users []uuid.UUID) (map[uuid.UUID]string, error)
	SetRole(ctx context.Context, org, user uuid.UUID, role *string) error
}

type AcceptInvitationDeps struct {
	DB          *pgtx.DB
	Members     *repositories.MemberRepository
	Invitations *repositories.InvitationRepository
	Roles       OrganizationRoles
}

type AcceptInvitationUseCase struct{ d AcceptInvitationDeps }

func NewAcceptInvitation(d AcceptInvitationDeps) *AcceptInvitationUseCase {
	return &AcceptInvitationUseCase{d: d}
}

type Acceptor struct {
	ID    uuid.UUID
	Email string
}

func (u *AcceptInvitationUseCase) Run(ctx context.Context, hash [32]byte, who Acceptor) (membershiptypes.Invitation, error) {
	var out membershiptypes.Invitation
	err := u.d.DB.InTx(ctx, func(ctx context.Context) error {
		found, err := u.d.Invitations.Lock(ctx, hash)
		if err != nil {
			return err
		}
		if !found {
			return apperr.ErrNotFound
		}
		inv, err := u.d.Invitations.ByHash(ctx, hash)
		if err != nil || inv == nil {
			return apperr.Wrap(err)
		}
		if inv.State() != membershiptypes.Pending {
			return membershiptypes.NotPending
		}
		if !strings.EqualFold(inv.Email, strings.TrimSpace(who.Email)) {
			return membershiptypes.EmailMismatch
		}
		var addedBy *uuid.UUID
		if inv.InvitedBy != nil {
			addedBy = &inv.InvitedBy.ID
		}
		if _, err := u.d.Members.Add(ctx, inv.OrganizationID, who.ID, membershiptypes.SourceInvitation, addedBy); err != nil {
			return err
		}
		roles, err := u.d.Roles.Roles(ctx, inv.OrganizationID, []uuid.UUID{who.ID})
		if err != nil {
			return err
		}
		var current *membershiptypes.Role
		if r, ok := roles[who.ID]; ok {
			role := membershiptypes.Role(r)
			current = &role
		}
		role := string(membershiptypes.Higher(current, inv.Role))
		if err := u.d.Roles.SetRole(ctx, inv.OrganizationID, who.ID, &role); err != nil {
			return err
		}
		if err := u.d.Invitations.Accept(ctx, inv.ID, who.ID); err != nil {
			return err
		}
		out = *inv
		return nil
	})
	return out, err
}
