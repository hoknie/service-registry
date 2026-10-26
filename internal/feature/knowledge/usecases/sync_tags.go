package usecases

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/knowledge/repositories"
	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/pkg/pgtx"
)

type SyncTagsInput struct {
	Project uuid.UUID
	Picked  []knowledgetypes.Tag
	Release string
}

type SyncTagsUseCase struct {
	db                 *pgtx.DB
	tagRepository      *repositories.TagRepository
	snapshotRepository *repositories.SnapshotRepository
}

func NewSyncTags(db *pgtx.DB, tags *repositories.TagRepository, snapshots *repositories.SnapshotRepository) *SyncTagsUseCase {
	return &SyncTagsUseCase{db: db, tagRepository: tags, snapshotRepository: snapshots}
}

func (u *SyncTagsUseCase) Run(ctx context.Context, in SyncTagsInput) error {
	return repositories.Err(u.db.InTx(ctx, func(ctx context.Context) error {
		removed, err := u.tagRepository.Replace(ctx, in.Project, in.Picked)
		if err != nil {
			return err
		}
		if err := u.snapshotRepository.DeleteTags(ctx, in.Project, removed); err != nil {
			return err
		}
		return u.tagRepository.SetRelease(ctx, in.Project, in.Release)
	}))
}
