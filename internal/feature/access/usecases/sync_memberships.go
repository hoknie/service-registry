package usecases

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/pkg/pgtx"
)

type SyncMembershipsInput struct {
	UserID  uuid.UUID
	Source  accesstypes.MembershipSource
	Groups  []string
	Editors []string
}

type SyncMembershipsUseCase struct {
	db              *pgtx.DB
	groupRepository *repositories.GroupRepository
}

func NewSyncMemberships(db *pgtx.DB, groups *repositories.GroupRepository) *SyncMembershipsUseCase {
	return &SyncMembershipsUseCase{db: db, groupRepository: groups}
}

func (u *SyncMembershipsUseCase) Run(ctx context.Context, in SyncMembershipsInput) error {
	keys := accesstypes.GroupNameKeys(in.Groups)
	editors := accesstypes.GroupNameKeys(in.Editors)
	groupIDs, memberIDs := newIDs(len(in.Groups)), newIDs(len(in.Groups))
	return u.db.InTx(ctx, func(ctx context.Context) error {
		if len(in.Groups) > 0 {
			if err := u.groupRepository.InsertGlobalGroups(ctx, groupIDs, in.Groups); err != nil {
				return err
			}
			if err := u.groupRepository.InsertManagedMembers(ctx, in.UserID, in.Source, memberIDs, keys); err != nil {
				return err
			}
		}
		if err := u.groupRepository.DeleteStaleManagedMembers(ctx, in.UserID, in.Source, keys); err != nil {
			return err
		}
		if err := u.groupRepository.InsertManagedEditors(ctx, in.UserID, in.Source, editors); err != nil {
			return err
		}
		return u.groupRepository.DeleteStaleManagedEditors(ctx, in.UserID, in.Source, editors)
	})
}

func newIDs(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.Must(uuid.NewV7())
	}
	return out
}
