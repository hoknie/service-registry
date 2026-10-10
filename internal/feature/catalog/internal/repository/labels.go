package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/catalog"
)

func collectSuggestions(rows pgx.Rows, err error) ([]catalog.LabelSuggestion, error) {
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (catalog.LabelSuggestion, error) {
		var s catalog.LabelSuggestion
		return s, r.Scan(&s.Value, &s.Count)
	})
	if err != nil {
		return nil, dbErr(err)
	}
	if out == nil {
		out = []catalog.LabelSuggestion{}
	}
	return out, nil
}

func (s *Nodes) LabelKeys(ctx context.Context, userID uuid.UUID, all bool, prefix string) ([]catalog.LabelSuggestion, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
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
		readable AS (
			SELECT n.id
			FROM nodes n
			WHERE $2
				OR n.id IN (SELECT node_id FROM bound)
			UNION
			SELECT c.id
			FROM nodes c
			JOIN readable r ON c.parent_id = r.id
		)
		SELECT k.key, count(*)
		FROM nodes n
		JOIN readable r ON r.id = n.id
		CROSS JOIN LATERAL jsonb_object_keys(n.labels) AS k(key)
		WHERE k.key ILIKE $3 ESCAPE '\'
		GROUP BY k.key
		ORDER BY count(*) DESC, k.key
		LIMIT 20`, userID, all, prefix)
	return collectSuggestions(rows, err)
}

func (s *Nodes) LabelValues(ctx context.Context, userID uuid.UUID, all bool, key, prefix string) ([]catalog.LabelSuggestion, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
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
		readable AS (
			SELECT n.id
			FROM nodes n
			WHERE $2
				OR n.id IN (SELECT node_id FROM bound)
			UNION
			SELECT c.id
			FROM nodes c
			JOIN readable r ON c.parent_id = r.id
		)
		SELECT n.labels ->> $3, count(*)
		FROM nodes n
		JOIN readable r ON r.id = n.id
		WHERE n.labels ? $3
			AND (n.labels ->> $3) ILIKE $4 ESCAPE '\'
		GROUP BY n.labels ->> $3
		ORDER BY count(*) DESC, n.labels ->> $3
		LIMIT 20`, userID, all, key, prefix)
	return collectSuggestions(rows, err)
}
