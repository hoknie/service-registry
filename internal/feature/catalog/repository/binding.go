package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/catalog"
	"svc-registry/internal/platform/postgres"
)

type Bindings struct{ db *postgres.DB }

func NewBindings(db *postgres.DB) *Bindings { return &Bindings{db: db} }

func scanBinding(row pgx.Row) (catalog.Binding, error) {
	var b catalog.Binding
	var isGroup bool
	var role string
	if err := row.Scan(&b.ID, &b.NodeID, &b.NodeName, &isGroup, &b.SubjectID, &b.SubjectName, &role, &b.CreatedAt); err != nil {
		return catalog.Binding{}, err
	}
	b.SubjectKind = catalog.SubjectUser
	if isGroup {
		b.SubjectKind = catalog.SubjectGroup
	}
	r, ok := catalog.ParseRole(role)
	if !ok {
		return catalog.Binding{}, internal("role", role)
	}
	b.Role = r
	return b, nil
}

func (s *Bindings) List(ctx context.Context, nodeIDs []uuid.UUID) ([]catalog.Binding, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT b.id, b.node_id, n.name AS node_name, b.user_id IS NULL AS is_group,
			COALESCE(b.user_id, b.group_id) AS subject_id, COALESCE(u.email, g.name) AS subject_name, b.role,
			rfc3339(b.created_at) AS created_at
		FROM role_bindings b
		JOIN nodes n ON n.id = b.node_id
		LEFT JOIN users u ON u.id = b.user_id
		LEFT JOIN groups g ON g.id = b.group_id
		WHERE b.node_id = ANY($1)
		ORDER BY array_position($1, b.node_id), b.user_id IS NULL, lower(COALESCE(u.email, g.name)), b.id`, nodeIDs)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (catalog.Binding, error) { return scanBinding(r) })
	if err != nil {
		return nil, dbErr(err)
	}
	return out, nil
}

func (s *Bindings) Put(ctx context.Context, b catalog.NewBinding) (*catalog.Binding, error) {
	got, err := scanBinding(s.db.From(ctx).QueryRow(ctx, `
		WITH s AS (
			SELECT u.id AS user_id, NULL::uuid AS group_id
			FROM users u
			WHERE $3 = 'user'
				AND lower(u.email) = lower($4)
			UNION ALL
			SELECT NULL, g.id
			FROM groups g
			WHERE $3 = 'group'
				AND lower(g.name) = lower($4)
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
		LEFT JOIN groups g ON g.id = b.group_id`, b.ID, b.NodeID, string(b.SubjectKind), b.Subject, b.Role.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &got, nil
}

func (s *Bindings) Delete(ctx context.Context, nodeID, id uuid.UUID) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM role_bindings
		WHERE node_id = $1
			AND id = $2`, nodeID, id)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return catalog.ErrNotFound
	}
	return nil
}
