package repositories

import (
	"context"

	"github.com/google/uuid"

	pollingtypes "svc-registry/internal/feature/polling/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type PollingRepository struct{ db *pgtx.DB }

func NewPollingRepository(db *pgtx.DB) *PollingRepository {
	return &PollingRepository{db: db}
}

func (s *PollingRepository) Chain(ctx context.Context, nodeID uuid.UUID) ([]pollingtypes.ChainNode, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(chainRow.chain), `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, 0 AS lvl
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT n.id, n.parent_id, up.lvl + 1
			FROM nodes n
			JOIN up ON n.id = up.parent_id
		)
		SELECT n.id, n.kind, n.name, p.interval_secs, p.timeout_secs, p.mode
		FROM up
		JOIN nodes n ON n.id = up.id
		LEFT JOIN node_polling p ON p.node_id = n.id
		ORDER BY up.lvl`, nodeID)
}

func (s *PollingRepository) Put(ctx context.Context, nodeID uuid.UUID, in pollingtypes.NodeSettings) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO node_polling (node_id, interval_secs, timeout_secs, mode)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (node_id)
		DO UPDATE SET interval_secs = $2, timeout_secs = $3, mode = $4, updated_at = now()`,
		nodeID, in.IntervalSecs, in.TimeoutSecs, modeParam(in.Mode))
	return err
}

func (s *PollingRepository) Delete(ctx context.Context, nodeID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM node_polling
		WHERE node_id = $1`, nodeID)
	return err
}
