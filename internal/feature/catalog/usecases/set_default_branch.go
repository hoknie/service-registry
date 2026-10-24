package usecases

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog/repositories"
	"svc-registry/pkg/pgtx"
)

type SetDefaultBranchUseCase struct {
	db               *pgtx.DB
	branchRepository *repositories.BranchRepository
}

func NewSetDefaultBranch(db *pgtx.DB, branches *repositories.BranchRepository) *SetDefaultBranchUseCase {
	return &SetDefaultBranchUseCase{db: db, branchRepository: branches}
}

func (u *SetDefaultBranchUseCase) Run(ctx context.Context, projectID uuid.UUID, name *string) error {
	return u.db.InTx(ctx, func(ctx context.Context) error {
		if err := u.branchRepository.ClearDefault(ctx, projectID, name); err != nil {
			return err
		}
		if name == nil {
			return nil
		}
		return u.branchRepository.MarkDefault(ctx, uuid.Must(uuid.NewV7()), projectID, *name)
	})
}
