package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type NodeRepository struct{ db *pgtx.DB }

func NewNodeRepository(db *pgtx.DB) *NodeRepository { return &NodeRepository{db: db} }

type chainRow struct {
	nodeRow
	BoundRank *int32 `db:"bound_rank"`
	Navigable bool   `db:"navigable"`
}

type walkRow struct {
	Total    int64  `db:"total"`
	Depth    *int32 `db:"depth"`
	Readable *bool  `db:"readable"`
	nodeRow
}

func (s *NodeRepository) one(ctx context.Context, query string, args ...any) (*catalogtypes.Node, error) {
	rows, _ := s.db.From(ctx).Query(ctx, query, args...)
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[nodeRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	n, err := row.node()
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *NodeRepository) Get(ctx context.Context, id uuid.UUID) (*catalogtypes.Node, error) {
	return s.one(ctx, `
		SELECT n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at
		FROM nodes n
		WHERE n.id = $1`, id)
}

func (s *NodeRepository) ChildBySlug(ctx context.Context, parent *uuid.UUID, slug string) (*catalogtypes.Node, error) {
	return s.one(ctx, `
		SELECT n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at
		FROM nodes n
		WHERE n.parent_id IS NOT DISTINCT FROM $1
			AND n.slug = $2`, parent, slug)
}

func (s *NodeRepository) Chain(ctx context.Context, userID, id uuid.UUID) (*catalogtypes.NodeChain, error) {
	rows, _ := s.db.From(ctx).Query(ctx, `
		WITH RECURSIVE bound AS (
			SELECT node_id, role
			FROM role_bindings
			WHERE user_id = $1
			UNION ALL
			SELECT b.node_id, b.role
			FROM role_bindings b
			JOIN group_members gm ON gm.group_id = b.group_id
			WHERE gm.user_id = $1
		),
		lineage AS (
			SELECT n.id, n.parent_id
			FROM nodes n
			WHERE n.id IN (SELECT node_id FROM bound)
			UNION
			SELECT p.id, p.parent_id
			FROM nodes p
			JOIN lineage l ON p.id = l.parent_id
		),
		up AS (
			SELECT n.*, 0 AS lvl
			FROM nodes n
			WHERE n.id = $2
			UNION ALL
			SELECT p.*, up.lvl + 1
			FROM nodes p
			JOIN up ON p.id = up.parent_id
		)
		SELECT n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at, (
			SELECT max(CASE b.role WHEN 'admin' THEN 3 WHEN 'editor' THEN 2 ELSE 1 END)
			FROM bound b
			WHERE b.node_id = n.id
		) AS bound_rank, EXISTS (SELECT 1 FROM lineage WHERE lineage.id = $2) AS navigable
		FROM up n
		ORDER BY n.lvl`, userID, id)
	found, err := pgx.CollectRows(rows, pgx.RowToStructByName[chainRow])
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	if len(found) == 0 {
		return nil, nil
	}
	chain := catalogtypes.NodeChain{Navigable: found[0].Navigable}
	for _, r := range found {
		n, err := r.node()
		if err != nil {
			return nil, err
		}
		chain.Nodes = append(chain.Nodes, catalogtypes.ChainNode{Node: n, Bound: roleOfRank(r.BoundRank)})
	}
	return &chain, nil
}

func (s *NodeRepository) Walk(ctx context.Context, w catalogtypes.Walk) ([]catalogtypes.WalkNode, uint64, error) {
	depth := int32(min(w.Depth, uint32(1<<31-1)))
	rows, _ := s.db.From(ctx).Query(ctx, `
		WITH RECURSIVE bound AS (
			SELECT node_id, role
			FROM role_bindings
			WHERE user_id = $1
			UNION ALL
			SELECT b.node_id, b.role
			FROM role_bindings b
			JOIN group_members gm ON gm.group_id = b.group_id
			WHERE gm.user_id = $1
		),
		lineage AS (
			SELECT n.id, n.parent_id
			FROM nodes n
			WHERE n.id IN (SELECT node_id FROM bound)
			UNION
			SELECT p.id, p.parent_id
			FROM nodes p
			JOIN lineage l ON p.id = l.parent_id
		),
		walk AS (
			SELECT n.id, 1 AS depth, ($2 OR n.id IN (SELECT node_id FROM bound)) AS readable
			FROM nodes n
			WHERE ($3::uuid IS NULL AND n.parent_id IS NULL) OR n.parent_id = $3
			UNION ALL
			SELECT c.id, w.depth + 1, (w.readable OR c.id IN (SELECT node_id FROM bound))
			FROM walk w
			JOIN nodes c ON c.parent_id = w.id
			WHERE w.depth < $4
				AND (w.readable OR w.id IN (SELECT id FROM lineage))
		),
		visible AS (
			SELECT w.depth, w.readable, n.*
			FROM walk w
			JOIN nodes n ON n.id = w.id
			WHERE w.readable OR w.id IN (SELECT id FROM lineage)
		)
		SELECT t.total, p.*
		FROM (SELECT count(*) AS total FROM visible) t
		LEFT JOIN LATERAL (
			SELECT n.depth, n.readable, n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels,
				n.forge, n.repo_url, n.default_branch, n.cluster_observation, rfc3339(n.created_at) AS created_at,
				rfc3339(n.updated_at) AS updated_at
			FROM visible n
			ORDER BY n.depth, CASE n.kind WHEN 'organization' THEN 0 WHEN 'folder' THEN 1 ELSE 2 END,
				lower(n.name), n.id
			LIMIT $5
			OFFSET $6
		) p ON true`, w.UserID, w.RootReadable, w.Root, depth, int64(w.Limit), bigint(w.Offset))
	found, err := pgx.CollectRows(rows, pgx.RowToStructByName[walkRow])
	if err != nil {
		return nil, 0, postgres.Translate(err, constraints)
	}
	var total int64
	nodes := []catalogtypes.WalkNode{}
	for _, r := range found {
		total = r.Total
		if r.ID == nil {
			continue
		}
		n, err := r.node()
		if err != nil {
			return nil, 0, err
		}
		nodes = append(nodes, catalogtypes.WalkNode{Node: n, Depth: *r.Depth, Readable: *r.Readable})
	}
	return nodes, uint64(max(total, 0)), nil
}

func bigint(n uint64) int64 {
	if n > 1<<63-1 {
		return 1<<63 - 1
	}
	return int64(n)
}
