package catalog

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/catalog"
	"svc-registry/internal/postgres"
)

type BindingStore struct{ pool *pgxpool.Pool }

func NewBindingStore(pool *pgxpool.Pool) *BindingStore { return &BindingStore{pool: pool} }

var bindingColumns = "b.id, b.node_id, n.name AS node_name, b.user_id IS NULL AS is_group, " +
	"COALESCE(b.user_id, b.group_id) AS subject_id, COALESCE(u.email, g.name) AS subject_name, " +
	"b.role, " + postgres.RFC3339("b.created_at") + " AS created_at FROM "

const bindingJoins = " b JOIN nodes n ON n.id = b.node_id LEFT JOIN users u ON u.id = b.user_id " +
	"LEFT JOIN groups g ON g.id = b.group_id"

var (
	bindingList = "SELECT " + bindingColumns + "role_bindings" + bindingJoins +
		" WHERE b.node_id = ANY($1) ORDER BY array_position($1, b.node_id), " +
		"b.user_id IS NULL, lower(COALESCE(u.email, g.name)), b.id"

	bindingPut = "WITH s AS (SELECT u.id AS user_id, NULL::uuid AS group_id FROM users u " +
		"WHERE $3 = 'user' AND lower(u.email) = lower($4) " +
		"UNION ALL SELECT NULL, g.id FROM groups g WHERE $3 = 'group' AND lower(g.name) = lower($4)), " +
		"ins AS (INSERT INTO role_bindings (id, node_id, user_id, group_id, role) " +
		"SELECT $1, $2, s.user_id, s.group_id, $5 FROM s " +
		"ON CONFLICT (node_id, user_id, group_id) DO UPDATE SET role = EXCLUDED.role, updated_at = now() " +
		"RETURNING *) SELECT " + bindingColumns + "ins" + bindingJoins

	bindingDelete = "DELETE FROM role_bindings WHERE node_id = $1 AND id = $2"
)

func scanBinding(row pgx.Row) (domain.Binding, error) {
	var b domain.Binding
	var isGroup bool
	var role string
	if err := row.Scan(&b.ID, &b.NodeID, &b.NodeName, &isGroup, &b.SubjectID, &b.SubjectName, &role, &b.CreatedAt); err != nil {
		return domain.Binding{}, err
	}
	b.SubjectKind = domain.SubjectUser
	if isGroup {
		b.SubjectKind = domain.SubjectGroup
	}
	r, ok := domain.ParseRole(role)
	if !ok {
		return domain.Binding{}, internal("role", role)
	}
	b.Role = r
	return b, nil
}

func (s *BindingStore) List(ctx context.Context, nodeIDs []uuid.UUID) ([]domain.Binding, error) {
	rows, err := s.pool.Query(ctx, bindingList, nodeIDs)
	if err != nil {
		return nil, dbErr(err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Binding, error) { return scanBinding(r) })
	if err != nil {
		return nil, dbErr(err)
	}
	return out, nil
}

func (s *BindingStore) Put(ctx context.Context, b domain.NewBinding) (*domain.Binding, error) {
	got, err := scanBinding(s.pool.QueryRow(ctx, bindingPut, b.ID, b.NodeID, string(b.SubjectKind), b.Subject, b.Role.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &got, nil
}

func (s *BindingStore) Delete(ctx context.Context, nodeID, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, bindingDelete, nodeID, id)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
