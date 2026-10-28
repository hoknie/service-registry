package usecases

import (
	"context"

	"svc-registry/internal/feature/skills/repositories"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/pkg/pgtx"
)

type ImportSourceUseCase struct {
	db      *pgtx.DB
	folders *repositories.FolderRepository
	sources *repositories.SourceRepository
}

func NewImportSource(db *pgtx.DB, folders *repositories.FolderRepository, sources *repositories.SourceRepository) *ImportSourceUseCase {
	return &ImportSourceUseCase{db: db, folders: folders, sources: sources}
}

func (u *ImportSourceUseCase) Run(ctx context.Context, folder skillstypes.NewFolder, src skillstypes.Source) error {
	return repositories.Err(u.db.InTx(ctx, func(ctx context.Context) error {
		if err := u.folders.Lock(ctx); err != nil {
			return err
		}
		if folder.ParentID != nil {
			chain, err := u.folders.Chain(ctx, *folder.ParentID)
			if err != nil {
				return err
			}
			if len(chain) >= skillstypes.MaxFolderDepth {
				return skillstypes.FolderTooDeep
			}
		}
		if err := u.folders.Insert(ctx, folder); err != nil {
			return err
		}
		return u.sources.Save(ctx, src)
	}))
}
