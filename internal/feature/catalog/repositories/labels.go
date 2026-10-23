package repositories

import (
	"context"

	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/postgres"
)

func scanSuggestion(r pgx.CollectableRow) (catalogtypes.LabelSuggestion, error) {
	var s catalogtypes.LabelSuggestion
	return s, r.Scan(&s.Value, &s.Count)
}

func suggestions(out []catalogtypes.LabelSuggestion, err error) ([]catalogtypes.LabelSuggestion, error) {
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []catalogtypes.LabelSuggestion{}
	}
	return out, nil
}

func (s *NodeRepository) LabelKeys(ctx context.Context, v catalogtypes.Viewer, all bool, prefix string) ([]catalogtypes.LabelSuggestion, error) {
	return suggestions(postgres.List(ctx, s.db.From(ctx), constraints, scanSuggestion, `
		WITH RECURSIVE bound AS (
			SELECT node_id
			FROM viewer_bindings($1, $4::uuid[], $5::boolean)
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
		LIMIT 20`, v.UserID, all, prefix, v.Nodes, v.Admin))
}

func (s *NodeRepository) LabelValues(ctx context.Context, v catalogtypes.Viewer, all bool, key, prefix string) ([]catalogtypes.LabelSuggestion, error) {
	return suggestions(postgres.List(ctx, s.db.From(ctx), constraints, scanSuggestion, `
		WITH RECURSIVE bound AS (
			SELECT node_id
			FROM viewer_bindings($1, $5::uuid[], $6::boolean)
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
		LIMIT 20`, v.UserID, all, key, prefix, v.Nodes, v.Admin))
}
