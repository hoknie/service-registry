package usecases

import (
	"context"

	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/pkg/pgtx"
)

type SaveSnapshotInput struct {
	Snapshot knowledgetypes.NewSnapshot
	Keep     uint32
}

type SaveSnapshotUseCase struct {
	db                 *pgtx.DB
	snapshotRepository *repositories.SnapshotRepository
}

func NewSaveSnapshot(db *pgtx.DB, snapshots *repositories.SnapshotRepository) *SaveSnapshotUseCase {
	return &SaveSnapshotUseCase{db: db, snapshotRepository: snapshots}
}

func (u *SaveSnapshotUseCase) Run(ctx context.Context, in SaveSnapshotInput) error {
	n := in.Snapshot
	if n.RefKind == "" {
		n.RefKind = knowledgetypes.RefBranch
	}
	return repositories.Err(u.db.InTx(ctx, func(ctx context.Context) error {
		if n.Status == knowledgetypes.StatusFailed {
			if err := u.snapshotRepository.Add(ctx, n, n.Totals()); err != nil {
				return err
			}
			return u.snapshotRepository.DropFailed(ctx, n.ProjectID, n.RefKind, n.Branch, n.ID)
		}
		refreshed, err := u.refreshSameTree(ctx, n)
		if err != nil || refreshed {
			return err
		}
		return u.insert(ctx, n, in.Keep)
	}))
}

func (u *SaveSnapshotUseCase) refreshSameTree(ctx context.Context, n knowledgetypes.NewSnapshot) (bool, error) {
	latest, err := u.snapshotRepository.Latest(ctx, n.ProjectID, n.RefKind, n.Branch)
	if err != nil || latest == nil {
		return false, err
	}
	stored, err := u.snapshotRepository.TreeKeys(ctx, latest.ID)
	if err != nil || !n.SameTree(stored) {
		return false, err
	}
	if err := u.snapshotRepository.Refresh(ctx, latest.ID, n); err != nil {
		return false, err
	}
	return true, u.snapshotRepository.DropFailed(ctx, n.ProjectID, n.RefKind, n.Branch, latest.ID)
}

func (u *SaveSnapshotUseCase) insert(ctx context.Context, n knowledgetypes.NewSnapshot, keep uint32) error {
	if err := u.snapshotRepository.AddBlobs(ctx, n.Files); err != nil {
		return err
	}
	if err := u.snapshotRepository.Add(ctx, n, n.Totals()); err != nil {
		return err
	}
	if err := u.snapshotRepository.AddFiles(ctx, n); err != nil {
		return err
	}
	if err := u.snapshotRepository.DropFailed(ctx, n.ProjectID, n.RefKind, n.Branch, n.ID); err != nil {
		return err
	}
	return u.snapshotRepository.Trim(ctx, n.ProjectID, n.RefKind, n.Branch, int64(keep)+1)
}
