package usecases

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/pkg/pgtx"
)

type DeleteBranchUseCase struct {
	db               *pgtx.DB
	branchRepository *repositories.BranchRepository
}

func NewDeleteBranch(db *pgtx.DB, branches *repositories.BranchRepository) *DeleteBranchUseCase {
	return &DeleteBranchUseCase{db: db, branchRepository: branches}
}

func (u *DeleteBranchUseCase) Run(ctx context.Context, projectID uuid.UUID, name string) error {
	return u.db.InTx(ctx, func(ctx context.Context) error {
		isDefault, err := u.branchRepository.DefaultForUpdate(ctx, projectID, name)
		if err != nil {
			return err
		}
		if isDefault == nil {
			return apperr.ErrNotFound
		}
		if *isDefault {
			return catalogtypes.ConflictBranchIsDefault
		}
		return u.branchRepository.Delete(ctx, projectID, name)
	})
}
