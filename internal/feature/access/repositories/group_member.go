package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/postgres"
)

func (s *GroupRepository) AddMember(ctx context.Context, id, groupID, userID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO group_members (id, group_id, user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (group_id, user_id)
		DO UPDATE SET source = 'manual'`, id, groupID, userID)
	return err
}

func (s *GroupRepository) RemoveManualMember(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	n, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		WITH gone AS (
			DELETE FROM group_members
			WHERE group_id = $1
				AND user_id = $2
				AND source = 'manual'
			RETURNING id
		),
		dropped AS (
			DELETE FROM group_contact_editors
			WHERE group_id = $1
				AND user_id = $2
				AND NOT EXISTS (
					SELECT 1
					FROM group_members
					WHERE group_id = $1
						AND user_id = $2
						AND source <> 'manual'
				)
		)
		SELECT count(*)
		FROM gone`, groupID, userID)
	return n > 0, err
}

func (s *GroupRepository) MemberSource(ctx context.Context, groupID, userID uuid.UUID) (*string, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[string], `
		SELECT source
		FROM group_members
		WHERE group_id = $1
			AND user_id = $2`, groupID, userID)
}

func (s *GroupRepository) InsertGlobalGroups(ctx context.Context, ids []uuid.UUID, names []string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO groups (id, name)
		SELECT g.id, g.name
		FROM unnest($1::uuid[], $2::text[]) AS g(id, name)
		WHERE NOT EXISTS (SELECT 1 FROM groups x WHERE x.organization_id IS NULL AND lower(x.name) = lower(g.name))
		ON CONFLICT
		DO NOTHING`, ids, names)
	return err
}

func (s *GroupRepository) InsertManagedMembers(ctx context.Context, userID uuid.UUID, source accesstypes.MembershipSource, ids []uuid.UUID, keys []string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO group_members (id, group_id, user_id, source)
		SELECT m.id, x.id, $1, $2
		FROM unnest($3::uuid[], $4::text[]) AS m(id, name)
		JOIN groups x ON x.organization_id IS NULL AND lower(x.name) = m.name
		ON CONFLICT (group_id, user_id)
		DO NOTHING`, userID, string(source), ids, keys)
	return err
}

func (s *GroupRepository) DeleteStaleManagedMembers(ctx context.Context, userID uuid.UUID, source accesstypes.MembershipSource, keys []string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM group_members m USING groups x
		WHERE m.group_id = x.id
			AND m.user_id = $1
			AND m.source = $2
			AND NOT (lower(x.name) = ANY($3::text[]))`, userID, string(source), keys)
	return err
}

func (s *GroupRepository) InsertManagedEditors(ctx context.Context, userID uuid.UUID, source accesstypes.MembershipSource, keys []string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO group_contact_editors (group_id, user_id, source)
		SELECT m.group_id, m.user_id, $2
		FROM group_members m
		JOIN groups x ON x.id = m.group_id
		WHERE m.user_id = $1
			AND x.organization_id IS NULL
			AND lower(x.name) = ANY($3::text[])
		ON CONFLICT (group_id, user_id)
		DO NOTHING`, userID, string(source), keys)
	return err
}

func (s *GroupRepository) DeleteStaleManagedEditors(ctx context.Context, userID uuid.UUID, source accesstypes.MembershipSource, keys []string) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM group_contact_editors e
		WHERE e.user_id = $1
			AND e.source = $2
			AND NOT EXISTS (
				SELECT 1
				FROM group_members m
				JOIN groups x ON x.id = m.group_id
				WHERE m.group_id = e.group_id
					AND m.user_id = e.user_id
					AND x.organization_id IS NULL
					AND lower(x.name) = ANY($3::text[])
			)`, userID, string(source), keys)
	return err
}

func (s *GroupRepository) Candidates(ctx context.Context, groupID uuid.UUID, pattern string, exclude *uuid.UUID, limit int64) ([]accesstypes.MemberCandidate, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (accesstypes.MemberCandidate, error) {
		var c accesstypes.MemberCandidate
		err := r.Scan(&c.ID, &c.DisplayName, &c.Email)
		return c, err
	}, `
		SELECT u.id, u.display_name, u.email
		FROM users u
		WHERE u.status = 'active'
			AND NOT u.is_service
			AND u.id IS DISTINCT FROM $3
			AND NOT EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = $1 AND m.user_id = u.id)
			AND (u.display_name ILIKE $2 ESCAPE '\' OR u.email ILIKE $2 ESCAPE '\')
		ORDER BY lower(u.display_name), u.email
		LIMIT $4`, groupID, pattern, exclude, limit)
}
