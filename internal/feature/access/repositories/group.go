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
			(SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count, g.on_call_url,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at`, g.ID, g.Name)
}

func (s *GroupRepository) List(ctx context.Context, page accesstypes.PageRequest) (accesstypes.Page[accesstypes.Group], error) {
	items, err := postgres.List(ctx, s.db.From(ctx), constraints, scanGroup, `
		SELECT g.id, g.name, (SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count, g.on_call_url,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at
		FROM groups g
		ORDER BY lower(g.name), g.id
		LIMIT $1
		OFFSET $2`, int64(page.Limit), int64(page.Offset))
	if err == nil {
		items, err = withContacts(ctx, s.db.From(ctx), items)
	}
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
	groups, err := postgres.List(ctx, s.db.From(ctx), constraints, scanGroup, `
		SELECT g.id, g.name, (SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count, g.on_call_url,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at
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
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at
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
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at`, id, name)
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

func (s *GroupRepository) AddMember(ctx context.Context, id, groupID, userID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO group_members (id, group_id, user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (group_id, user_id)
		DO UPDATE SET source = 'manual'`, id, groupID, userID)
	return err
}

func (s *GroupRepository) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
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

func (s *GroupRepository) SyncManaged(ctx context.Context, userID uuid.UUID, source accesstypes.MembershipSource, groups, editors []string) error {
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
		if _, err := postgres.Exec(ctx, tx, constraints, `
			DELETE FROM group_members m USING groups x
			WHERE m.group_id = x.id
				AND m.user_id = $1
				AND m.source = $2
				AND NOT (lower(x.name) = ANY($3::text[]))`, userID, string(source), lower); err != nil {
			return err
		}
		return syncEditors(ctx, tx, userID, source, editors)
	})
	return postgres.Translate(err, constraints)
}

func syncEditors(ctx context.Context, tx pgtx.Querier, userID uuid.UUID, source accesstypes.MembershipSource, editors []string) error {
	lower := make([]string, len(editors))
	for i, g := range editors {
		lower[i] = strings.ToLower(g)
	}
	if _, err := postgres.Exec(ctx, tx, constraints, `
		INSERT INTO group_contact_editors (group_id, user_id, source)
		SELECT m.group_id, m.user_id, $2
		FROM group_members m
		JOIN groups x ON x.id = m.group_id
		WHERE m.user_id = $1
			AND lower(x.name) = ANY($3::text[])
		ON CONFLICT (group_id, user_id)
		DO NOTHING`, userID, string(source), lower); err != nil {
		return err
	}
	_, err := postgres.Exec(ctx, tx, constraints, `
		DELETE FROM group_contact_editors e
		WHERE e.user_id = $1
			AND e.source = $2
			AND NOT EXISTS (
				SELECT 1
				FROM group_members m
				JOIN groups x ON x.id = m.group_id
				WHERE m.group_id = e.group_id
					AND m.user_id = e.user_id
					AND lower(x.name) = ANY($3::text[])
			)`, userID, string(source), lower)
	return err
}
