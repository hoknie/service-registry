package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	gatewaytypes "svc-registry/internal/feature/gateway/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type AccessRepository struct{ db *pgtx.DB }

func NewAccessRepository(db *pgtx.DB) *AccessRepository { return &AccessRepository{db: db} }

func (s *AccessRepository) Groups(ctx context.Context, server uuid.UUID) ([]gatewaytypes.GroupRef, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (gatewaytypes.GroupRef, error) {
		var g gatewaytypes.GroupRef
		err := r.Scan(&g.ID, &g.Name)
		return g, err
	}, `
		SELECT g.id, g.name
		FROM mcp_server_access a
		JOIN groups g ON g.id = a.group_id
		WHERE a.server_id = $1
		ORDER BY lower(g.name), g.id`, server)
}

func (s *AccessRepository) Users(ctx context.Context, server uuid.UUID) ([]gatewaytypes.UserRef, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (gatewaytypes.UserRef, error) {
		var u gatewaytypes.UserRef
		err := r.Scan(&u.ID, &u.Email, &u.Name)
		return u, err
	}, `
		SELECT u.id, u.email, u.display_name
		FROM mcp_server_access a
		JOIN users u ON u.id = a.user_id
		WHERE a.server_id = $1
		ORDER BY lower(u.email), u.id`, server)
}

func (s *AccessRepository) ValidSubjects(ctx context.Context, organization *uuid.UUID, groups, users []uuid.UUID) (bool, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT (SELECT count(*) FROM groups WHERE id = ANY($2) AND (organization_id IS NULL OR organization_id = $1)) = cardinality($2::uuid[])
			AND (SELECT count(*) FROM users WHERE id = ANY($3)) = cardinality($3::uuid[])`, organization, groups, users)
}

func (s *AccessRepository) Replace(ctx context.Context, server uuid.UUID, everyone bool, groups, users []uuid.UUID) error {
	if err := postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE mcp_servers
		SET everyone = $2, updated_at = now()
		WHERE id = $1`, server, everyone); err != nil {
		return err
	}
	if _, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM mcp_server_access
		WHERE server_id = $1`, server); err != nil {
		return err
	}
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO mcp_server_access (server_id, group_id, user_id)
		SELECT $1::uuid, g, NULL::uuid
		FROM unnest($2::uuid[]) AS g
		UNION ALL
		SELECT $1::uuid, NULL::uuid, u
		FROM unnest($3::uuid[]) AS u`, server, groups, users)
	return err
}

func (s *AccessRepository) Summaries(ctx context.Context, servers []uuid.UUID) ([]AccessSubject, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, postgres.Struct(func(r AccessSubject) AccessSubject { return r }), `
		SELECT a.server_id, a.group_id, a.user_id, COALESCE(g.name, u.email) AS label
		FROM mcp_server_access a
		LEFT JOIN groups g ON g.id = a.group_id
		LEFT JOIN users u ON u.id = a.user_id
		WHERE a.server_id = ANY($1)
		ORDER BY a.server_id, a.group_id IS NULL, lower(COALESCE(g.name, u.email))`, servers)
}

type AccessSubject struct {
	ServerID uuid.UUID  `db:"server_id"`
	GroupID  *uuid.UUID `db:"group_id"`
	UserID   *uuid.UUID `db:"user_id"`
	Label    string     `db:"label"`
}
