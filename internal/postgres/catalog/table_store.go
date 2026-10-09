package catalog

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	domain "svc-registry/internal/catalog"
)

type tableRow struct {
	Total    int64  `db:"total"`
	Depth    *int32 `db:"depth"`
	Readable *bool  `db:"readable"`
	Match    *bool  `db:"match"`
	Children *int64 `db:"children"`
	nodeRow
}

func (s *NodeStore) Table(ctx context.Context, w domain.Walk) ([]domain.TableNode, uint64, error) {
	rows, _ := s.pool.Query(ctx, `
		WITH RECURSIVE bound AS (
			SELECT node_id
			FROM role_bindings
			WHERE user_id = $1
			UNION ALL
			SELECT b.node_id
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
		visible AS (
			SELECT ($2 OR n.id IN (SELECT node_id FROM bound)) AS readable, n.*
			FROM nodes n
			WHERE (($3::uuid IS NULL AND n.parent_id IS NULL) OR n.parent_id = $3)
				AND ($2 OR n.id IN (SELECT node_id FROM bound) OR n.id IN (SELECT id FROM lineage))
		)
		SELECT t.total, p.*
		FROM (SELECT count(*) AS total FROM visible) t
		LEFT JOIN LATERAL (
			SELECT 1 AS depth, n.readable, true AS match,
				(
					SELECT count(*)
					FROM nodes c
					WHERE c.parent_id = n.id
						AND (n.readable OR c.id IN (SELECT node_id FROM bound) OR c.id IN (SELECT id FROM lineage))
				) AS children,
				n.id, n.kind, n.parent_id, n.slug, n.name, n.description, n.labels, n.forge, n.repo_url, n.default_branch,
				n.cluster_observation, rfc3339(n.created_at) AS created_at, rfc3339(n.updated_at) AS updated_at
			FROM visible n
			ORDER BY CASE n.kind WHEN 'organization' THEN 0 WHEN 'folder' THEN 1 ELSE 2 END, lower(n.name), n.id
			LIMIT $4
			OFFSET $5
		) p ON true`, w.UserID, w.RootReadable, w.Root, int64(w.Limit), bigint(w.Offset))
	return tableNodes(rows)
}

func (s *NodeStore) Search(ctx context.Context, q domain.Search) ([]domain.TableNode, uint64, error) {
	values := map[string]string{}
	keys := []string{}
	for _, l := range q.Filter.Labels {
		if l.Value == nil {
			keys = append(keys, l.Key)
			continue
		}
		if v, ok := values[l.Key]; ok && v != *l.Value {
			return []domain.TableNode{}, 0, nil
		}
		values[l.Key] = *l.Value
	}
	pairs, err := json.Marshal(values)
	if err != nil {
		return nil, 0, err
	}
	rows, _ := s.pool.Query(ctx, `
		WITH RECURSIVE bound AS (
			SELECT node_id
			FROM role_bindings
			WHERE user_id = $1
			UNION ALL
			SELECT b.node_id
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
		ranked AS (
			SELECT id, row_number() OVER (
				PARTITION BY parent_id
				ORDER BY CASE kind WHEN 'organization' THEN 0 WHEN 'folder' THEN 1 ELSE 2 END, lower(name), id
			) AS rk
			FROM nodes
		),
		walk AS (
			SELECT n.id, 1 AS depth, ($2 OR n.id IN (SELECT node_id FROM bound)) AS readable, ARRAY[n.id] AS ancestors,
				ARRAY[r.rk] AS path
			FROM nodes n
			JOIN ranked r ON r.id = n.id
			WHERE ($3::uuid IS NULL AND n.parent_id IS NULL) OR n.parent_id = $3
			UNION ALL
			SELECT c.id, w.depth + 1, (w.readable OR c.id IN (SELECT node_id FROM bound)), w.ancestors || c.id, w.path || r.rk
			FROM walk w
			JOIN nodes c ON c.parent_id = w.id
			JOIN ranked r ON r.id = c.id
			WHERE w.depth < 64
				AND (w.readable OR w.id IN (SELECT id FROM lineage))
		),
		visible AS (
			SELECT w.depth, w.readable, w.ancestors, w.path, n.*
			FROM walk w
			JOIN nodes n ON n.id = w.id
			WHERE w.readable OR w.id IN (SELECT id FROM lineage)
		),
		hits AS (
			SELECT v.id, v.ancestors, v.path
			FROM visible v
			WHERE ($4 = '' OR strpos(lower(v.name), lower($4)) > 0 OR strpos(v.slug, lower($4)) > 0)
				AND ($5 = '' OR v.kind = $5)
				AND ($6::jsonb = '{}'::jsonb OR (v.readable AND v.labels @> $6::jsonb))
				AND (cardinality($7::text[]) = 0 OR (v.readable AND v.labels ?& $7::text[]))
				AND ($8::uuid[] IS NULL OR (v.readable AND v.id = ANY($8)))
		),
		kept AS (
			SELECT id, ancestors
			FROM hits
			ORDER BY path
			LIMIT $9
		),
		shown AS (
			SELECT DISTINCT unnest(ancestors) AS id
			FROM kept
		)
		SELECT (SELECT count(*) FROM hits) AS total, v.depth, v.readable, v.id IN (SELECT id FROM kept) AS match,
			(
				SELECT count(*)
				FROM nodes c
				WHERE c.parent_id = v.id
					AND (v.readable OR c.id IN (SELECT node_id FROM bound) OR c.id IN (SELECT id FROM lineage))
			) AS children,
			v.id, v.kind, v.parent_id, v.slug, v.name, v.description, v.labels, v.forge, v.repo_url, v.default_branch,
			v.cluster_observation, rfc3339(v.created_at) AS created_at, rfc3339(v.updated_at) AS updated_at
		FROM visible v
		WHERE v.id IN (SELECT id FROM shown)
		ORDER BY v.path`, q.UserID, q.RootReadable, q.Root, q.Filter.Q, string(q.Filter.Kind), string(pairs), keys, q.Only,
		int64(q.Limit))
	return tableNodes(rows)
}

func tableNodes(rows pgx.Rows) ([]domain.TableNode, uint64, error) {
	found, err := pgx.CollectRows(rows, pgx.RowToStructByName[tableRow])
	if err != nil {
		return nil, 0, dbErr(err)
	}
	var total int64
	nodes := []domain.TableNode{}
	for _, r := range found {
		total = r.Total
		if r.ID == nil {
			continue
		}
		n, err := r.node()
		if err != nil {
			return nil, 0, err
		}
		nodes = append(nodes, domain.TableNode{WalkNode: domain.WalkNode{Node: n, Depth: *r.Depth, Readable: *r.Readable},
			Children: *r.Children, Match: *r.Match})
	}
	return nodes, uint64(max(total, 0)), nil
}
