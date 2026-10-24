package repositories

import (
	"context"

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
		INSERT INTO groups AS g (id, name, organization_id)
		VALUES ($1, $2, $3)
		RETURNING g.id, g.name,
			(SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count, g.on_call_url,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at, g.organization_id,
			(SELECT o.name FROM nodes o WHERE o.id = g.organization_id) AS organization_name`, g.ID, g.Name, g.Organization)
}

func (s *GroupRepository) List(ctx context.Context, page accesstypes.PageRequest, org *uuid.UUID) (accesstypes.Page[accesstypes.Group], error) {
	items, err := postgres.List(ctx, s.db.From(ctx), constraints, scanGroup, `
		SELECT g.id, g.name, (SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count, g.on_call_url,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at, g.organization_id,
			(SELECT o.name FROM nodes o WHERE o.id = g.organization_id) AS organization_name
		FROM groups g
		WHERE $3::uuid IS NULL
			OR g.organization_id = $3
		ORDER BY lower(g.name), g.organization_id NULLS FIRST, g.id
		LIMIT $1
		OFFSET $2`, int64(page.Limit), int64(page.Offset), org)
	if err == nil {
		items, err = withContacts(ctx, s.db.From(ctx), items)
	}
	if err != nil {
		return accesstypes.Page[accesstypes.Group]{}, err
	}
	total, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM groups
		WHERE $1::uuid IS NULL
			OR organization_id = $1`, org)
	if err != nil {
		return accesstypes.Page[accesstypes.Group]{}, err
	}
	return accesstypes.Page[accesstypes.Group]{Items: items, Total: uint64(max(total, 0)), Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *GroupRepository) ForUser(ctx context.Context, userID uuid.UUID) ([]accesstypes.Group, error) {
	groups, err := postgres.List(ctx, s.db.From(ctx), constraints, scanGroup, `
		SELECT g.id, g.name, (SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count, g.on_call_url,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at, g.organization_id,
			(SELECT o.name FROM nodes o WHERE o.id = g.organization_id) AS organization_name
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id
		WHERE gm.user_id = $1
		ORDER BY lower(g.name), g.id`, userID)
	if err != nil {
		return nil, err
	}
	return withContacts(ctx, s.db.From(ctx), groups)
}

func (s *GroupRepository) Find(ctx context.Context, id uuid.UUID) (*accesstypes.GroupDetails, error) {
	group, err := postgres.Find(ctx, s.db.From(ctx), constraints, scanGroup, `
		SELECT g.id, g.name, (SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count, g.on_call_url,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at, g.organization_id,
			(SELECT o.name FROM nodes o WHERE o.id = g.organization_id) AS organization_name
		FROM groups g
		WHERE g.id = $1`, id)
	if err != nil || group == nil {
		return nil, err
	}
	withGroup, err := withContacts(ctx, s.db.From(ctx), []accesstypes.Group{*group})
	if err != nil {
		return nil, err
	}
	members, err := postgres.List(ctx, s.db.From(ctx), constraints, scanMember, `
		SELECT u.id, u.email, u.display_name, u.status, m.source,
			(SELECT e.source FROM group_contact_editors e WHERE e.group_id = m.group_id AND e.user_id = m.user_id) AS contact_editor_source
		FROM group_members m
		JOIN users u ON u.id = m.user_id
		WHERE m.group_id = $1
		ORDER BY u.email`, id)
	if err != nil {
		return nil, err
	}
	return &accesstypes.GroupDetails{Group: withGroup[0], Members: members}, nil
}

func (s *GroupRepository) Rename(ctx context.Context, id uuid.UUID, name string) (accesstypes.Group, error) {
	group, err := postgres.Find(ctx, s.db.From(ctx), constraints, scanGroup, `
		UPDATE groups AS g
		SET name = $2, updated_at = now()
		WHERE g.id = $1
		RETURNING g.id, g.name,
			(SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count, g.on_call_url,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at, g.organization_id,
			(SELECT o.name FROM nodes o WHERE o.id = g.organization_id) AS organization_name`, id, name)
	if err != nil {
		return accesstypes.Group{}, err
	}
	if group == nil {
		return accesstypes.Group{}, apperr.ErrNotFound
	}
	out, err := withContacts(ctx, s.db.From(ctx), []accesstypes.Group{*group})
	if err != nil {
		return accesstypes.Group{}, err
	}
	return out[0], nil
}

func (s *GroupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		DELETE FROM groups
		WHERE id = $1`, id)
}

func (s *GroupRepository) ByIDs(ctx context.Context, ids []uuid.UUID) ([]accesstypes.Group, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, scanGroup, `
		SELECT g.id, g.name, (SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count, g.on_call_url,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at, g.organization_id,
			(SELECT o.name FROM nodes o WHERE o.id = g.organization_id) AS organization_name
		FROM groups g
		WHERE g.id = ANY($1)
		ORDER BY lower(g.name), g.id`, ids)
}
