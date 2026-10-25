package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	knowledgetypes "svc-registry/internal/feature/knowledge/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type PatternRepository struct{ db *pgtx.DB }

func NewPatternRepository(db *pgtx.DB) *PatternRepository { return &PatternRepository{db: db} }

func (s *PatternRepository) Chain(ctx context.Context, nodeID uuid.UUID) ([]knowledgetypes.ChainNode, error) {
	chain, err := postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (knowledgetypes.ChainNode, error) {
		var n knowledgetypes.ChainNode
		err := r.Scan(&n.ID, &n.Kind, &n.Name, &n.Own.IncludeSet, &n.Own.Include, &n.Own.Exclude, &n.Own.Branches, &n.Own.APIs, &n.Own.Refresh,
			&n.Own.APIPaths, &n.Own.RefreshEvents)
		return n, err
	}, `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, kind, name, 0 AS lvl
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT n.id, n.parent_id, n.kind, n.name, up.lvl + 1
			FROM nodes n
			JOIN up ON n.id = up.parent_id
		)
		SELECT up.id, up.kind, up.name, COALESCE(p.include_set, false), p.include, p.exclude, p.branches, p.apis, p.refresh, p.api_paths, p.refresh_events
		FROM up
		LEFT JOIN knowledge_patterns p ON p.node_id = up.id
		ORDER BY up.lvl`, nodeID)
	if err != nil {
		return nil, err
	}
	if len(chain) == 0 {
		return nil, apperr.ErrNotFound
	}
	return chain, nil
}

func (s *PatternRepository) Delete(ctx context.Context, nodeID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM knowledge_patterns
		WHERE node_id = $1`, nodeID)
	return err
}

func (s *PatternRepository) Upsert(ctx context.Context, nodeID uuid.UUID, in knowledgetypes.NodeSettings) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO knowledge_patterns (node_id, include_set, include, exclude, branches, apis, refresh, api_paths, refresh_events)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (node_id)
		DO UPDATE SET include_set = $2, include = $3, exclude = $4, branches = $5, apis = $6, refresh = $7, api_paths = $8,
			refresh_events = $9, updated_at = now()`,
		nodeID, in.IncludeSet, in.Include, in.Exclude, in.Branches, in.APIs, in.Refresh, in.APIPaths, in.RefreshEvents)
	return err
}
