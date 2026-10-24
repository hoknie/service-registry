package usecases

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/ownership/repositories"
	ownershiptypes "svc-registry/internal/feature/ownership/types"
	"svc-registry/pkg/pgtx"
)

type ReplaceOwnersInput struct {
	NodeID uuid.UUID
	Owners []ownershiptypes.NewOwner
}

type ReplaceOwnersUseCase struct {
	db              *pgtx.DB
	ownerRepository *repositories.OwnerRepository
}

func NewReplaceOwners(db *pgtx.DB, owners *repositories.OwnerRepository) *ReplaceOwnersUseCase {
	return &ReplaceOwnersUseCase{db: db, ownerRepository: owners}
}

func (u *ReplaceOwnersUseCase) Run(ctx context.Context, in ReplaceOwnersInput) error {
	return u.db.InTx(ctx, func(ctx context.Context) error {
		if err := u.ownerRepository.LockNode(ctx, in.NodeID); err != nil {
			return err
		}
		resolved, err := u.ownerRepository.Resolve(ctx, in.NodeID, in.Owners)
		if err != nil {
			return err
		}
		inherited, err := u.ownerRepository.AncestorSubjects(ctx, in.NodeID)
		if err != nil {
			return err
		}
		own, err := ownershiptypes.OwnSubjects(resolved, inherited)
		if err != nil {
			return err
		}
		if err := u.ownerRepository.DeleteOwn(ctx, in.NodeID); err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(own))
		for i := range ids {
			ids[i] = uuid.Must(uuid.NewV7())
		}
		return u.ownerRepository.InsertOwn(ctx, in.NodeID, ids, own)
	})
}
