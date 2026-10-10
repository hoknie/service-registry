package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/postgres"
)

type Nodes struct{ db *postgres.DB }

func NewNodes(db *postgres.DB) *Nodes { return &Nodes{db: db} }

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

func (s *Nodes) one(ctx context.Context, query string, args ...any) (*catalog.Node, error) {
	rows, _ := s.db.From(ctx).Query(ctx, query, args...)
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

func (s *Nodes) Get(ctx context.Context, id uuid.UUID) (*catalog.Node, error) {
	return s.one(ctx, `
		SELECT n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at
		FROM nodes n
		WHERE n.id = $1`, id)
}

func (s *Nodes) ChildBySlug(ctx context.Context, parent *uuid.UUID, slug string) (*catalog.Node, error) {
	return s.one(ctx, `
		SELECT n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at
		FROM nodes n
		WHERE n.parent_id IS NOT DISTINCT FROM $1
			AND n.slug = $2`, parent, slug)
}

func (s *Nodes) Chain(ctx context.Context, userID, id uuid.UUID) (*catalog.NodeChain, error) {
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
		return nil, dbErr(err)
	}
	if len(found) == 0 {
		return nil, nil
	}
	chain := catalog.NodeChain{Navigable: found[0].Navigable}
	for _, r := range found {
		n, err := r.node()
		if err != nil {
			return nil, err
		}
		chain.Nodes = append(chain.Nodes, catalog.ChainNode{Node: n, Bound: roleOfRank(r.BoundRank)})
	}
	return &chain, nil
}

func (s *Nodes) Walk(ctx context.Context, w catalog.Walk) ([]catalog.WalkNode, uint64, error) {
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
		return nil, 0, dbErr(err)
	}
	var total int64
	nodes := []catalog.WalkNode{}
	for _, r := range found {
		total = r.Total
		if r.ID == nil {
			continue
		}
		n, err := r.node()
		if err != nil {
			return nil, 0, err
		}
		nodes = append(nodes, catalog.WalkNode{Node: n, Depth: *r.Depth, Readable: *r.Readable})
	}
	return nodes, uint64(max(total, 0)), nil
}

func forgeText(f *catalog.Forge) *string {
	if f == nil {
		return nil
	}
	s := string(*f)
	return &s
}

func (s *Nodes) Insert(ctx context.Context, n catalog.NewNode) (catalog.Node, error) {
	rows, _ := s.db.From(ctx).Query(ctx, `
		INSERT INTO nodes AS n (id, kind, parent_id, slug, name, description, labels, forge, repo_url,
			default_branch, cluster_observation)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, COALESCE($11, true))
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, n.ID, string(n.Kind), n.ParentID, n.Slug, n.Name,
		n.Description, labelsParam(n.Labels), forgeText(n.Repo.Forge), n.Repo.RepoURL, n.Repo.DefaultBranch, n.ClusterObservation)
	node, err := oneNode(rows)
	return node, dbErr(err)
}

func (s *Nodes) Update(ctx context.Context, id uuid.UUID, c catalog.NodeChanges) (catalog.Node, error) {
	var labels *catalog.Labels
	if c.Labels != nil {
		labels = &c.Labels
	}
	rows, _ := s.db.From(ctx).Query(ctx, `
		UPDATE nodes AS n
		SET slug = COALESCE($2, n.slug), name = COALESCE($3, n.name),
			description = COALESCE($4, n.description), labels = COALESCE($5::jsonb, n.labels),
			forge = CASE WHEN $6 THEN $7 ELSE n.forge END, repo_url = CASE WHEN $8 THEN $9 ELSE n.repo_url END,
			default_branch = CASE WHEN $10 THEN $11 ELSE n.default_branch END,
			cluster_observation = COALESCE($12, n.cluster_observation), updated_at = now()
		WHERE n.id = $1
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, id, c.Slug, c.Name, c.Description, labels,
		c.Forge.Set, forgeText(c.Forge.Value), c.RepoURL.Set, c.RepoURL.Value,
		c.DefaultBranch.Set, c.DefaultBranch.Value, c.ClusterObservation)
	node, err := oneNode(rows)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.Node{}, catalog.ErrNotFound
	}
	return node, dbErr(err)
}

func (s *Nodes) Move(ctx context.Context, id uuid.UUID, parent *uuid.UUID) (catalog.Node, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return catalog.Node{}, dbErr(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(7316001)`); err != nil {
		return catalog.Node{}, dbErr(err)
	}
	if parent != nil {
		var cycle bool
		if err := tx.QueryRow(ctx, `
			WITH RECURSIVE up AS (
				SELECT id, parent_id
				FROM nodes
				WHERE id = $1
				UNION ALL
				SELECT p.id, p.parent_id
				FROM nodes p
				JOIN up ON p.id = up.parent_id
			)
			SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, *parent, id).Scan(&cycle); err != nil {
			return catalog.Node{}, dbErr(err)
		}
		if cycle {
			return catalog.Node{}, catalog.ConflictCycle
		}
	}
	rows, _ := tx.Query(ctx, `
		UPDATE nodes AS n
		SET parent_id = $2, updated_at = now()
		WHERE n.id = $1
		RETURNING n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url,
			n.default_branch, n.cluster_observation, rfc3339(n.created_at) AS created_at,
			rfc3339(n.updated_at) AS updated_at`, id, parent)
	node, err := oneNode(rows)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.Node{}, catalog.ErrNotFound
	}
	if err != nil {
		return catalog.Node{}, dbErr(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.Node{}, dbErr(err)
	}
	return node, nil
}

func (s *Nodes) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM nodes
		WHERE id = $1`, id)
	if err != nil {
		if constraint(err) == "nodes_parent_id_fkey" {
			return catalog.ConflictNodeNotEmpty
		}
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return catalog.ErrNotFound
	}
	return nil
}

func bigint(n uint64) int64 {
	if n > 1<<63-1 {
		return 1<<63 - 1
	}
	return int64(n)
}
