package usecases

import (
	"context"

	"github.com/google/uuid"

	"svc-registry/internal/feature/catalog/repositories"
	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/pkg/pgtx"
)

type MoveNodeUseCase struct {
	db             *pgtx.DB
	nodeRepository *repositories.NodeRepository
}

func NewMoveNode(db *pgtx.DB, nodes *repositories.NodeRepository) *MoveNodeUseCase {
	return &MoveNodeUseCase{db: db, nodeRepository: nodes}
}

func (u *MoveNodeUseCase) Run(ctx context.Context, id uuid.UUID, parent *uuid.UUID) (catalogtypes.Node, error) {
	var moved catalogtypes.Node
	err := u.db.InTx(ctx, func(ctx context.Context) error {
		if err := u.nodeRepository.LockTree(ctx); err != nil {
			return err
		}
		if parent != nil {
			cycle, err := u.nodeRepository.IsAncestorOrSelf(ctx, id, *parent)
			if err != nil {
				return err
			}
			if cycle {
				return catalogtypes.ConflictCycle
			}
		}
		var err error
		moved, err = u.nodeRepository.SetParent(ctx, id, parent)
		return err
	})
	return moved, err
}
