package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/catalog"
)

type NodeStore struct{ pool *pgxpool.Pool }

func NewNodeStore(pool *pgxpool.Pool) *NodeStore { return &NodeStore{pool: pool} }

var (
	nodeChain = "WITH RECURSIVE " + boundCTE +
		", up AS (SELECT n.*, 0 AS lvl FROM nodes n WHERE n.id = $2 " +
		"UNION ALL SELECT p.*, up.lvl + 1 FROM nodes p JOIN up ON p.id = up.parent_id) " +
		"SELECT " + nodeColumns +
		", (SELECT max(CASE b.role WHEN 'admin' THEN 3 WHEN 'editor' THEN 2 ELSE 1 END) " +
		"FROM bound b WHERE b.node_id = n.id) AS bound_rank, " +
		"EXISTS (SELECT 1 FROM lineage WHERE lineage.id = $2) AS navigable " +
		"FROM up n ORDER BY n.lvl"

	nodeWalk = "WITH RECURSIVE " + boundCTE +
		", walk AS (SELECT n.id, 1 AS depth, ($2 OR n.id IN (SELECT node_id FROM bound)) AS readable " +
		"FROM nodes n WHERE ($3::uuid IS NULL AND n.parent_id IS NULL) OR n.parent_id = $3 " +
		"UNION ALL SELECT c.id, w.depth + 1, (w.readable OR c.id IN (SELECT node_id FROM bound)) " +
		"FROM walk w JOIN nodes c ON c.parent_id = w.id " +
		"WHERE w.depth < $4 AND (w.readable OR w.id IN (SELECT id FROM lineage))), " +
		"visible AS (SELECT w.depth, w.readable, n.* FROM walk w JOIN nodes n ON n.id = w.id " +
		"WHERE w.readable OR w.id IN (SELECT id FROM lineage)) " +
		"SELECT t.total, p.* FROM (SELECT count(*) AS total FROM visible) t LEFT JOIN LATERAL (" +
		"SELECT n.depth, n.readable, " + nodeColumns +
		" FROM visible n ORDER BY n.depth, " + kindRank + ", lower(n.name), n.id LIMIT $5 OFFSET $6) p ON true"

	nodeInsert = "INSERT INTO nodes AS n (id, kind, parent_id, slug, name, description, labels, forge, " +
		"repo_url, default_branch, cluster_observation) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, COALESCE($11, true)) RETURNING " + nodeColumns

	nodeUpdate = "UPDATE nodes AS n SET slug = COALESCE($2, n.slug), name = COALESCE($3, n.name), " +
		"description = COALESCE($4, n.description), labels = COALESCE($5::jsonb, n.labels), " +
		"forge = CASE WHEN $6 THEN $7 ELSE n.forge END, " +
		"repo_url = CASE WHEN $8 THEN $9 ELSE n.repo_url END, " +
		"default_branch = CASE WHEN $10 THEN $11 ELSE n.default_branch END, " +
		"cluster_observation = COALESCE($12, n.cluster_observation), " +
		"updated_at = now() WHERE n.id = $1 RETURNING " + nodeColumns

	nodeLockMoves        = "SELECT pg_advisory_xact_lock(7316001)"
	nodeIsAncestorOrSelf = "WITH RECURSIVE up AS (SELECT id, parent_id FROM nodes " +
		"WHERE id = $1 UNION ALL SELECT p.id, p.parent_id FROM nodes p JOIN up ON p.id = up.parent_id) " +
		"SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)"
	nodeMove   = "UPDATE nodes AS n SET parent_id = $2, updated_at = now() WHERE n.id = $1 RETURNING " + nodeColumns
	nodeDelete = "DELETE FROM nodes WHERE id = $1"
	nodeGet    = "SELECT " + nodeColumns + " FROM nodes n WHERE n.id = $1"
	nodeChild  = "SELECT " + nodeColumns + " FROM nodes n WHERE n.parent_id IS NOT DISTINCT FROM $1 AND n.slug = $2"
)

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

func (s *NodeStore) one(ctx context.Context, query string, args ...any) (*domain.Node, error) {
	rows, _ := s.pool.Query(ctx, query, args...)
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[nodeRow])
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	n, err := row.node()
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *NodeStore) Get(ctx context.Context, id uuid.UUID) (*domain.Node, error) {
	return s.one(ctx, nodeGet, id)
}

func (s *NodeStore) ChildBySlug(ctx context.Context, parent *uuid.UUID, slug string) (*domain.Node, error) {
	return s.one(ctx, nodeChild, parent, slug)
}

func (s *NodeStore) Chain(ctx context.Context, userID, id uuid.UUID) (*domain.NodeChain, error) {
	rows, _ := s.pool.Query(ctx, nodeChain, userID, id)
	found, err := pgx.CollectRows(rows, pgx.RowToStructByName[chainRow])
	if err != nil {
		return nil, dbErr(err)
	}
	if len(found) == 0 {
		return nil, nil
	}
	chain := domain.NodeChain{Navigable: found[0].Navigable}
	for _, r := range found {
		n, err := r.node()
		if err != nil {
			return nil, err
		}
		chain.Nodes = append(chain.Nodes, domain.ChainNode{Node: n, Bound: roleOfRank(r.BoundRank)})
	}
	return &chain, nil
}

func (s *NodeStore) Walk(ctx context.Context, w domain.Walk) ([]domain.WalkNode, uint64, error) {
	depth := int32(min(w.Depth, uint32(1<<31-1)))
	rows, _ := s.pool.Query(ctx, nodeWalk, w.UserID, w.RootReadable, w.Root, depth, int64(w.Limit), bigint(w.Offset))
	found, err := pgx.CollectRows(rows, pgx.RowToStructByName[walkRow])
	if err != nil {
		return nil, 0, dbErr(err)
	}
	var total int64
	nodes := []domain.WalkNode{}
	for _, r := range found {
		total = r.Total
		if r.ID == nil {
			continue
		}
		n, err := r.node()
		if err != nil {
			return nil, 0, err
		}
		nodes = append(nodes, domain.WalkNode{Node: n, Depth: *r.Depth, Readable: *r.Readable})
	}
	return nodes, uint64(max(total, 0)), nil
}

func forgeText(f *domain.Forge) *string {
	if f == nil {
		return nil
	}
	s := string(*f)
	return &s
}

func (s *NodeStore) Insert(ctx context.Context, n domain.NewNode) (domain.Node, error) {
	rows, _ := s.pool.Query(ctx, nodeInsert, n.ID, string(n.Kind), n.ParentID, n.Slug, n.Name,
		n.Description, labelsParam(n.Labels), forgeText(n.Repo.Forge), n.Repo.RepoURL, n.Repo.DefaultBranch, n.ClusterObservation)
	node, err := oneNode(rows)
	return node, dbErr(err)
}

func (s *NodeStore) Update(ctx context.Context, id uuid.UUID, c domain.NodeChanges) (domain.Node, error) {
	var labels *domain.Labels
	if c.Labels != nil {
		labels = &c.Labels
	}
	rows, _ := s.pool.Query(ctx, nodeUpdate, id, c.Slug, c.Name, c.Description, labels,
		c.Forge.Set, forgeText(c.Forge.Value), c.RepoURL.Set, c.RepoURL.Value,
		c.DefaultBranch.Set, c.DefaultBranch.Value, c.ClusterObservation)
	node, err := oneNode(rows)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Node{}, domain.ErrNotFound
	}
	return node, dbErr(err)
}

func (s *NodeStore) Move(ctx context.Context, id uuid.UUID, parent *uuid.UUID) (domain.Node, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Node{}, dbErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, nodeLockMoves); err != nil {
		return domain.Node{}, dbErr(err)
	}
	if parent != nil {
		var cycle bool
		if err := tx.QueryRow(ctx, nodeIsAncestorOrSelf, *parent, id).Scan(&cycle); err != nil {
			return domain.Node{}, dbErr(err)
		}
		if cycle {
			return domain.Node{}, domain.ConflictCycle
		}
	}
	rows, _ := tx.Query(ctx, nodeMove, id, parent)
	node, err := oneNode(rows)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Node{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Node{}, dbErr(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Node{}, dbErr(err)
	}
	return node, nil
}

func (s *NodeStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, nodeDelete, id)
	if err != nil {
		if constraint(err) == "nodes_parent_id_fkey" {
			return domain.ConflictNodeNotEmpty
		}
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func bigint(n uint64) int64 {
	if n > 1<<63-1 {
		return 1<<63 - 1
	}
	return int64(n)
}
