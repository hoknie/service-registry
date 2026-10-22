package repositories

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type GroupRepository struct{ db *pgtx.DB }

func NewGroupRepository(db *pgtx.DB) *GroupRepository { return &GroupRepository{db: db} }

func (s *GroupRepository) Insert(ctx context.Context, g accesstypes.NewGroup) (accesstypes.Group, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, scanGroup, `
		INSERT INTO groups AS g (id, name)
		VALUES ($1, $2)
		RETURNING g.id, g.name,
			(SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at`, g.ID, g.Name)
}

func (s *GroupRepository) List(ctx context.Context, page accesstypes.PageRequest) (accesstypes.Page[accesstypes.Group], error) {
	items, err := postgres.List(ctx, s.db.From(ctx), constraints, scanGroup, `
		SELECT g.id, g.name, (SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at
		FROM groups g
		ORDER BY lower(g.name), g.id
		LIMIT $1
		OFFSET $2`, int64(page.Limit), int64(page.Offset))
	if err != nil {
		return accesstypes.Page[accesstypes.Group]{}, err
	}
	total, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM groups`)
	if err != nil {
		return accesstypes.Page[accesstypes.Group]{}, err
	}
	return accesstypes.Page[accesstypes.Group]{Items: items, Total: uint64(max(total, 0)), Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *GroupRepository) ForUser(ctx context.Context, userID uuid.UUID) ([]accesstypes.Group, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, scanGroup, `
		SELECT g.id, g.name, (SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id
		WHERE gm.user_id = $1
		ORDER BY lower(g.name), g.id`, userID)
}

func (s *GroupRepository) Find(ctx context.Context, id uuid.UUID) (*accesstypes.GroupDetails, error) {
	group, err := postgres.Find(ctx, s.db.From(ctx), constraints, scanGroup, `
		SELECT g.id, g.name, (SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at
		FROM groups g
		WHERE g.id = $1`, id)
	if err != nil || group == nil {
		return nil, err
	}
	members, err := postgres.List(ctx, s.db.From(ctx), constraints, scanMember, `
		SELECT u.id, u.email, u.display_name, u.status, m.source
		FROM group_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.group_id = $1
		ORDER BY u.email`, id)
	if err != nil {
		return nil, err
	}
	return &accesstypes.GroupDetails{Group: *group, Members: members}, nil
}

func (s *GroupRepository) Rename(ctx context.Context, id uuid.UUID, name string) (accesstypes.Group, error) {
	group, err := postgres.Find(ctx, s.db.From(ctx), constraints, scanGroup, `
		UPDATE groups AS g
		SET name = $2, updated_at = now()
		WHERE g.id = $1
		RETURNING g.id, g.name,
			(SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at`, id, name)
	if err != nil {
		return accesstypes.Group{}, err
	}
	if group == nil {
		return accesstypes.Group{}, apperr.ErrNotFound
	}
	return *group, nil
}

func (s *GroupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM groups
		WHERE id = $1`, id)
}

func (s *GroupRepository) AddMember(ctx context.Context, id, groupID, userID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO group_members (id, group_id, user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (group_id, user_id)
		DO UPDATE SET source = 'manual'`, id, groupID, userID)
	return err
}

func (s *GroupRepository) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	n, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM group_members
		WHERE group_id = $1
			AND user_id = $2
			AND source = 'manual'`, groupID, userID)
	if err != nil || n > 0 {
		return err
	}
	source, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[string], `
		SELECT source
		FROM group_members
		WHERE group_id = $1
			AND user_id = $2`, groupID, userID)
	if err != nil {
		return err
	}
	if source != nil {
		return accesstypes.ConflictMembershipManaged
	}
	exists, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT EXISTS (SELECT 1 FROM groups WHERE id = $1)`, groupID)
	if err != nil {
		return err
	}
	if !exists {
		return apperr.ErrNotFound
	}
	return nil
}

func (s *GroupRepository) SyncManaged(ctx context.Context, userID uuid.UUID, source accesstypes.MembershipSource, groups []string) error {
	ids := make([]uuid.UUID, len(groups))
	memberIDs := make([]uuid.UUID, len(groups))
	lower := make([]string, len(groups))
	for i, g := range groups {
		ids[i], memberIDs[i] = uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
		lower[i] = strings.ToLower(g)
	}
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		if len(groups) > 0 {
			if _, err := postgres.Exec(ctx, tx, constraints, `
				INSERT INTO groups (id, name)
				SELECT g.id, g.name
				FROM unnest($1::uuid[], $2::text[]) AS g(id, name)
				WHERE NOT EXISTS (SELECT 1 FROM groups x WHERE lower(x.name) = lower(g.name))
				ON CONFLICT
				DO NOTHING`, ids, groups); err != nil {
				return err
			}
			if _, err := postgres.Exec(ctx, tx, constraints, `
				INSERT INTO group_members (id, group_id, user_id, source)
				SELECT m.id, x.id, $1, $2
				FROM unnest($3::uuid[], $4::text[]) AS m(id, name)
				JOIN groups x ON lower(x.name) = m.name
				ON CONFLICT (group_id, user_id)
				DO NOTHING`, userID, string(source), memberIDs, lower); err != nil {
				return err
			}
		}
		_, err := postgres.Exec(ctx, tx, constraints, `
			DELETE FROM group_members m USING groups x
			WHERE m.group_id = x.id
				AND m.user_id = $1
				AND m.source = $2
				AND NOT (lower(x.name) = ANY($3::text[]))`, userID, string(source), lower)
		return err
	})
	return postgres.Translate(err, constraints)
}
