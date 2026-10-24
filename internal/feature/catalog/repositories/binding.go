package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	catalogtypes "svc-registry/internal/feature/catalog/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type BindingRepository struct{ db *pgtx.DB }

func NewBindingRepository(db *pgtx.DB) *BindingRepository { return &BindingRepository{db: db} }

func (s *BindingRepository) List(ctx context.Context, nodeIDs []uuid.UUID) ([]catalogtypes.Binding, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, scanBinding, `
		SELECT b.id, b.node_id, n.name AS node_name, b.user_id IS NULL AS is_group,
			COALESCE(b.user_id, b.group_id) AS subject_id, COALESCE(u.email, g.name) AS subject_name, b.role,
			rfc3339(b.created_at) AS created_at
		FROM role_bindings b
		JOIN nodes n ON n.id = b.node_id
		LEFT JOIN users u ON u.id = b.user_id
		LEFT JOIN groups g ON g.id = b.group_id
		WHERE b.node_id = ANY($1)
		ORDER BY array_position($1, b.node_id), b.user_id IS NULL, lower(COALESCE(u.email, g.name)), b.id`, nodeIDs)
}

func (s *BindingRepository) Put(ctx context.Context, b catalogtypes.NewBinding) (*catalogtypes.Binding, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanBinding, `
		WITH s AS (
			SELECT u.id AS user_id, NULL::uuid AS group_id
			FROM users u
			WHERE $3 = 'user'
				AND lower(u.email) = lower($4)
			UNION ALL
			(
				SELECT NULL, g.id
				FROM groups g
				WHERE $3 = 'group'
					AND lower(g.name) = lower($4)
					AND (g.organization_id IS NULL OR g.organization_id = node_root($2))
				ORDER BY g.organization_id IS NULL
				LIMIT 1
			)
		),
		ins AS (
			INSERT INTO role_bindings (id, node_id, user_id, group_id, role)
			SELECT $1, $2, s.user_id, s.group_id, $5
			FROM s
			ON CONFLICT (node_id, user_id, group_id)
			DO UPDATE SET role = EXCLUDED.role, updated_at = now()
			RETURNING *
		)
		SELECT b.id, b.node_id, n.name AS node_name, b.user_id IS NULL AS is_group,
			COALESCE(b.user_id, b.group_id) AS subject_id, COALESCE(u.email, g.name) AS subject_name, b.role,
			rfc3339(b.created_at) AS created_at
		FROM ins b
		JOIN nodes n ON n.id = b.node_id
		LEFT JOIN users u ON u.id = b.user_id
		LEFT JOIN groups g ON g.id = b.group_id`, b.ID, b.NodeID, string(b.SubjectKind), b.Subject, b.Role.String())
}

func (s *BindingRepository) Delete(ctx context.Context, nodeID, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM role_bindings
		WHERE node_id = $1
			AND id = $2`, nodeID, id)
}

func (s *BindingRepository) GrantUser(ctx context.Context, id, nodeID, userID uuid.UUID, role catalogtypes.Role) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO role_bindings (id, node_id, user_id, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (node_id, user_id, group_id)
		DO UPDATE SET role = EXCLUDED.role, updated_at = now()`, id, nodeID, userID, role.String())
	return err
}

func (s *BindingRepository) GroupNodes(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	rows, err := postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) ([2]uuid.UUID, error) {
		var pair [2]uuid.UUID
		err := r.Scan(&pair[0], &pair[1])
		return pair, err
	}, `
		SELECT DISTINCT group_id, node_id
		FROM role_bindings
		WHERE group_id IS NOT NULL
			AND ($1::uuid[] IS NULL OR group_id = ANY($1))
		ORDER BY group_id, node_id`, groupIDs)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID][]uuid.UUID{}
	for _, pair := range rows {
		out[pair[0]] = append(out[pair[0]], pair[1])
	}
	return out, nil
}
