package usecases

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/pkg/pgtx"
)

type SyncHostingBranchesUseCase struct {
	db               *pgtx.DB
	branchRepository *repositories.BranchRepository
}

func NewSyncHostingBranches(db *pgtx.DB, branches *repositories.BranchRepository) *SyncHostingBranchesUseCase {
	return &SyncHostingBranchesUseCase{db: db, branchRepository: branches}
}

func (u *SyncHostingBranchesUseCase) Run(ctx context.Context, projectID uuid.UUID, branches []catalogtypes.RepoSyncBranch, complete bool) (bool, error) {
	moved := false
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		heads, err := u.branchRepository.Heads(ctx, projectID)
		if err != nil {
			return err
		}
		moved = catalogtypes.HeadsMoved(heads, branches)
		names := make([]string, 0, len(branches))
		for _, b := range branches {
			if err := u.branchRepository.UpsertRepoSync(ctx, uuid.Must(uuid.NewV7()), projectID, b.Name, catalogtypes.BranchSHA(&b.HeadSHA),
				b.Protected, b.CommittedAt); err != nil {
				return err
			}
			names = append(names, b.Name)
		}
		if !complete {
			return nil
		}
		return u.branchRepository.MarkRepoSyncGone(ctx, projectID, names)
	})
	return moved, err
}
