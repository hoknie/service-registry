package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type GroupContactRepository struct{ db *pgtx.DB }

func NewGroupContactRepository(db *pgtx.DB) *GroupContactRepository {
	return &GroupContactRepository{db: db}
}

type groupContact struct {
	groupID uuid.UUID
	contact accesstypes.Contact
}

func scanGroupContact(row pgx.CollectableRow) (groupContact, error) {
	var c groupContact
	err := row.Scan(&c.groupID, &c.contact.Kind, &c.contact.Value, &c.contact.Label)
	return c, err
}

func contactsOf(ctx context.Context, q pgtx.Querier, ids []uuid.UUID) (map[uuid.UUID][]accesstypes.Contact, error) {
	out := map[uuid.UUID][]accesstypes.Contact{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := postgres.List(ctx, q, constraints, scanGroupContact, `
		SELECT group_id, kind, value, label
		FROM group_contacts
		WHERE group_id = ANY($1)
		ORDER BY group_id, position`, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.groupID] = append(out[r.groupID], r.contact)
	}
	return out, nil
}

func withContacts(ctx context.Context, q pgtx.Querier, groups []accesstypes.Group) ([]accesstypes.Group, error) {
	ids := make([]uuid.UUID, len(groups))
	for i, g := range groups {
		ids[i] = g.ID
	}
	contacts, err := contactsOf(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		if c, ok := contacts[groups[i].ID]; ok {
			groups[i].Contacts = c
		}
	}
	return groups, nil
}

func (s *GroupContactRepository) SetOnCall(ctx context.Context, groupID uuid.UUID, onCallURL *string) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE groups
		SET on_call_url = $2, updated_at = now()
		WHERE id = $1`, groupID, onCallURL)
}

func (s *GroupContactRepository) ReplaceContacts(ctx context.Context, groupID uuid.UUID, ids []uuid.UUID, contacts []accesstypes.Contact) error {
	tx := s.db.From(ctx)
	if _, err := postgres.Exec(ctx, tx, constraints, `
		DELETE FROM group_contacts
		WHERE group_id = $1`, groupID); err != nil {
		return err
	}
	for i, c := range contacts {
		if _, err := postgres.Exec(ctx, tx, constraints, `
			INSERT INTO group_contacts (id, group_id, kind, value, label, position)
			VALUES ($1, $2, $3, $4, $5, $6)`, ids[i], groupID, c.Kind, c.Value, c.Label, int16(i)); err != nil {
			return err
		}
	}
	return nil
}

func (s *GroupContactRepository) Membership(ctx context.Context, groupID, userID uuid.UUID) (accesstypes.Membership, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, scanMembership, `
		SELECT EXISTS (SELECT 1 FROM groups WHERE id = $1),
			EXISTS (SELECT 1 FROM users WHERE id = $2),
			EXISTS (SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2),
			EXISTS (
				SELECT 1
				FROM group_members m
				JOIN group_contact_editors e ON e.group_id = m.group_id AND e.user_id = m.user_id
				WHERE m.group_id = $1
					AND m.user_id = $2
			)`, groupID, userID)
}

func (s *GroupContactRepository) GrantEditor(ctx context.Context, groupID, userID uuid.UUID) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO group_contact_editors (group_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (group_id, user_id)
		DO UPDATE SET source = 'manual'`, groupID, userID)
	return err
}

func (s *GroupContactRepository) RevokeManualEditor(ctx context.Context, groupID, userID uuid.UUID) (bool, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		WITH gone AS (
			DELETE FROM group_contact_editors
			WHERE group_id = $1
				AND user_id = $2
				AND source = 'manual'
		)
		SELECT EXISTS (
			SELECT 1
			FROM group_contact_editors
			WHERE group_id = $1
				AND user_id = $2
				AND source <> 'manual'
		)`, groupID, userID)
}

func (s *GroupContactRepository) OwnGroups(ctx context.Context, userID uuid.UUID) ([]accesstypes.OwnGroup, error) {
	rows, err := postgres.List(ctx, s.db.From(ctx), constraints, scanOwnGroup, `
		SELECT g.id, g.name, (SELECT count(*) FROM group_members m WHERE m.group_id = g.id) AS member_count, g.on_call_url,
			rfc3339(g.created_at) AS created_at, rfc3339(g.updated_at) AS updated_at, g.organization_id,
			(SELECT o.name FROM nodes o WHERE o.id = g.organization_id) AS organization_name,
			EXISTS (SELECT 1 FROM group_contact_editors e WHERE e.group_id = g.id AND e.user_id = $1) AS can_edit
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id
		WHERE gm.user_id = $1
		ORDER BY lower(g.name), g.id`, userID)
	if err != nil {
		return nil, err
	}
	groups := make([]accesstypes.Group, len(rows))
	for i, r := range rows {
		groups[i] = r.Group
	}
	groups, err = withContacts(ctx, s.db.From(ctx), groups)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Group = groups[i]
	}
	return rows, nil
}

type GroupPermissionRepository struct{ db *pgtx.DB }

func NewGroupPermissionRepository(db *pgtx.DB) *GroupPermissionRepository {
	return &GroupPermissionRepository{db: db}
}

func (s *GroupPermissionRepository) rights(ctx context.Context, sql string, args ...any) (accesstypes.Rights, error) {
	pairs, err := postgres.Pairs[string, []string](ctx, s.db.From(ctx), constraints, sql, args...)
	if err != nil {
		return nil, err
	}
	out := accesstypes.Rights{}
	for section, actions := range pairs {
		for _, a := range actions {
			out[accesstypes.Section(section)] = append(out[accesstypes.Section(section)], accesstypes.Action(a))
		}
	}
	return out, nil
}

func (s *GroupPermissionRepository) ForGroup(ctx context.Context, groupID uuid.UUID) (accesstypes.Rights, error) {
	return s.rights(ctx, `
		SELECT section, actions
		FROM group_permissions
		WHERE group_id = $1`, groupID)
}

func (s *GroupPermissionRepository) ForUser(ctx context.Context, userID uuid.UUID) (accesstypes.Rights, error) {
	return s.rights(ctx, `
		SELECT p.section, array_agg(DISTINCT a)
		FROM group_permissions p
		JOIN group_members m ON m.group_id = p.group_id
		JOIN users u ON u.id = m.user_id
		CROSS JOIN LATERAL unnest(p.actions) AS a
		WHERE m.user_id = $1
			AND u.status = 'active'
		GROUP BY p.section`, userID)
}

func (s *GroupPermissionRepository) Replace(ctx context.Context, groupID uuid.UUID, rights accesstypes.Rights) error {
	tx := s.db.From(ctx)
	if _, err := postgres.Exec(ctx, tx, constraints, `
		DELETE FROM group_permissions
		WHERE group_id = $1`, groupID); err != nil {
		return err
	}
	for section, actions := range rights {
		if len(actions) == 0 {
			continue
		}
		names := make([]string, len(actions))
		for i, a := range actions {
			names[i] = string(a)
		}
		if _, err := postgres.Exec(ctx, tx, constraints, `
			INSERT INTO group_permissions (group_id, section, actions)
			VALUES ($1, $2, $3)`, groupID, string(section), names); err != nil {
			return err
		}
	}
	return nil
}

func (s *GroupPermissionRepository) GroupHasAny(ctx context.Context, groupID uuid.UUID) (bool, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT EXISTS (SELECT 1 FROM group_permissions WHERE group_id = $1)`, groupID)
}

func (s *GroupPermissionRepository) UserHasAny(ctx context.Context, userID uuid.UUID) (bool, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT EXISTS (
			SELECT 1
			FROM group_permissions p
			JOIN group_members m ON m.group_id = p.group_id
			WHERE m.user_id = $1
		)`, userID)
}
