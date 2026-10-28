package usecases

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/membership/repositories"
	membershiptypes "svc-registry/internal/feature/membership/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
)

type KeepAdminDeps struct {
	DB      *pgtx.DB
	Members *repositories.MemberRepository
}

type KeepAdminUseCase struct{ d KeepAdminDeps }

func NewKeepAdmin(d KeepAdminDeps) *KeepAdminUseCase { return &KeepAdminUseCase{d: d} }

func (u *KeepAdminUseCase) Run(ctx context.Context, org uuid.UUID, change func(ctx context.Context) error) error {
	return u.d.DB.InTx(ctx, func(ctx context.Context) error {
		before, err := u.d.Members.Admins(ctx, org)
		if err != nil {
			return err
		}
		if err := change(ctx); err != nil {
			return err
		}
		after, err := u.d.Members.Admins(ctx, org)
		if err != nil {
			return err
		}
		if before > 0 && after == 0 {
			return membershiptypes.LastAdmin
		}
		return nil
	})
}

type RemoveMemberUseCase struct {
	keep    *KeepAdminUseCase
	members *repositories.MemberRepository
}

func NewRemoveMember(keep *KeepAdminUseCase, members *repositories.MemberRepository) *RemoveMemberUseCase {
	return &RemoveMemberUseCase{keep: keep, members: members}
}

func (u *RemoveMemberUseCase) Run(ctx context.Context, org, user uuid.UUID) error {
	return u.keep.Run(ctx, org, func(ctx context.Context) error {
		removed, err := u.members.Remove(ctx, org, user)
		if err != nil {
			return err
		}
		if !removed {
			return apperr.ErrNotFound
		}
		return nil
	})
}
