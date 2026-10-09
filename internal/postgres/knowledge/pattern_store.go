package knowledge

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/knowledge"
)

type PatternStore struct{ pool *pgxpool.Pool }

func NewPatternStore(pool *pgxpool.Pool) *PatternStore { return &PatternStore{pool: pool} }

func (s *PatternStore) Chain(ctx context.Context, nodeID uuid.UUID) ([]domain.ChainNode, error) {
	rows, err := s.pool.Query(ctx, `
		WITH RECURSIVE up AS (
			SELECT id, parent_id, kind, name, 0 AS lvl
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT n.id, n.parent_id, n.kind, n.name, up.lvl + 1
			FROM nodes n
			JOIN up ON n.id = up.parent_id
		)
		SELECT up.id, up.kind, up.name, COALESCE(p.include_set, false), p.include, p.exclude, p.branches
		FROM up
		LEFT JOIN knowledge_patterns p ON p.node_id = up.id
		ORDER BY up.lvl`, nodeID)
	if err != nil {
		return nil, dbErr(err)
	}
	defer rows.Close()
	var chain []domain.ChainNode
	for rows.Next() {
		var n domain.ChainNode
		if err := rows.Scan(&n.ID, &n.Kind, &n.Name, &n.Own.IncludeSet, &n.Own.Include, &n.Own.Exclude, &n.Own.Branches); err != nil {
			return nil, dbErr(err)
		}
		chain = append(chain, n)
	}
	if err := rows.Err(); err != nil {
		return nil, dbErr(err)
	}
	if len(chain) == 0 {
		return nil, domain.ErrNotFound
	}
	return chain, nil
}

func (s *PatternStore) Put(ctx context.Context, nodeID uuid.UUID, in domain.NodeSettings) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return dbErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if in.IsEmpty() {
		_, err = tx.Exec(ctx, `
			DELETE FROM knowledge_patterns
			WHERE node_id = $1`, nodeID)
	} else {
		include := in.Include
		if !in.IncludeSet {
			include = nil
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO knowledge_patterns (node_id, include_set, include, exclude, branches)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (node_id)
			DO UPDATE SET include_set = $2, include = $3, exclude = $4, branches = $5, updated_at = now()`,
			nodeID, in.IncludeSet, include, in.Exclude, in.Branches)
	}
	if err != nil {
		return dbErr(err)
	}
	if _, err := tx.Exec(ctx, `
		WITH RECURSIVE down AS (
			SELECT id
			FROM nodes
			WHERE id = $1
			UNION ALL
			SELECT n.id
			FROM nodes n
			JOIN down ON n.parent_id = down.id
		)
		UPDATE knowledge_settings
		SET next_run_at = now(), updated_at = now()
		WHERE project_id IN (SELECT id FROM down)`, nodeID); err != nil {
		return dbErr(err)
	}
	return dbErr(tx.Commit(ctx))
}
