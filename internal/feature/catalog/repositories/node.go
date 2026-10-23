package repositories

import (
	"context"

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
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanNode, query, args...)
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

func (s *NodeRepository) Chain(ctx context.Context, v catalogtypes.Viewer, id uuid.UUID) (*catalogtypes.NodeChain, error) {
	found, err := postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowToStructByName[chainRow], `
		WITH RECURSIVE bound AS (
			SELECT node_id, role
			FROM viewer_bindings($1, $3::uuid[], $4::boolean)
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
		ORDER BY n.lvl`, v.UserID, id, v.Nodes, v.Admin)
	if err != nil {
		return nil, err
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
	found, err := postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowToStructByName[walkRow], `
		WITH RECURSIVE bound AS (
			SELECT node_id, role
			FROM viewer_bindings($1, $7::uuid[], $8::boolean)
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
		) p ON true`, w.Viewer.UserID, w.RootReadable, w.Root, depth, int64(w.Limit), bigint(w.Offset), w.Viewer.Nodes, w.Viewer.Admin)
	if err != nil {
		return nil, 0, err
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

func (s *NodeRepository) Paths(ctx context.Context, ids []uuid.UUID) ([]catalogtypes.NodePath, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (catalogtypes.NodePath, error) {
		var p catalogtypes.NodePath
		err := r.Scan(&p.ID, &p.Kind, &p.Name, &p.Path)
		return p, err
	}, `
		WITH RECURSIVE up AS (
			SELECT n.id AS start, n.parent_id, n.slug, 0 AS lvl
			FROM nodes n
			WHERE n.id = ANY($1)
			UNION ALL
			SELECT up.start, p.parent_id, p.slug, up.lvl + 1
			FROM up
			JOIN nodes p ON p.id = up.parent_id
		)
		SELECT n.id, n.kind, n.name, (
			SELECT string_agg(u.slug, '/' ORDER BY u.lvl DESC)
			FROM up u
			WHERE u.start = n.id
		) AS path
		FROM nodes n
		WHERE n.id = ANY($1)`, ids)
}

func (s *NodeRepository) Readable(ctx context.Context, v catalogtypes.Viewer, all bool, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	return postgres.Set[uuid.UUID](ctx, s.db.From(ctx), constraints, `
		WITH RECURSIVE bound AS (
			SELECT node_id
			FROM viewer_bindings($1, $2::uuid[], $5::boolean)
		),
		up AS (
			SELECT n.id AS start, n.id, n.parent_id
			FROM nodes n
			WHERE n.id = ANY($3)
			UNION ALL
			SELECT up.start, p.id, p.parent_id
			FROM up
			JOIN nodes p ON p.id = up.parent_id
		)
		SELECT DISTINCT up.start
		FROM up
		WHERE $4
			OR up.id IN (SELECT node_id FROM bound)`, v.UserID, v.Nodes, ids, all, v.Admin)
}

func bigint(n uint64) int64 {
	if n > 1<<63-1 {
		return 1<<63 - 1
	}
	return int64(n)
}
