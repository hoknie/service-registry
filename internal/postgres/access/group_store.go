package access

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/access"
)

type GroupStore struct{ pool *pgxpool.Pool }

func NewGroupStore(pool *pgxpool.Pool) *GroupStore { return &GroupStore{pool: pool} }

var (
	groupInsert  = "INSERT INTO groups AS g (id, name) VALUES ($1, $2) RETURNING " + groupColumns
	groupList    = "SELECT " + groupColumns + " FROM groups g ORDER BY lower(g.name), g.id LIMIT $1 OFFSET $2"
	groupCount   = "SELECT count(*) FROM groups"
	groupFind    = "SELECT " + groupColumns + " FROM groups g WHERE g.id = $1"
	groupMembers = "SELECT u.id, u.email, u.display_name, u.status, m.source FROM group_members m " +
		"JOIN users u ON u.id = m.user_id WHERE m.group_id = $1 ORDER BY u.email"
	groupRename    = "UPDATE groups AS g SET name = $2, updated_at = now() WHERE g.id = $1 RETURNING " + groupColumns
	groupDelete    = "DELETE FROM groups WHERE id = $1"
	groupAddMember = "INSERT INTO group_members (id, group_id, user_id) VALUES ($1, $2, $3) " +
		"ON CONFLICT (group_id, user_id) DO UPDATE SET source = 'manual'"
	groupMemberSource = "SELECT source FROM group_members WHERE group_id = $1 AND user_id = $2"
	groupRemoveMember = "DELETE FROM group_members WHERE group_id = $1 AND user_id = $2 AND source = 'manual'"
	groupEnsure       = "INSERT INTO groups (id, name) SELECT g.id, g.name FROM unnest($1::uuid[], $2::text[]) AS g(id, name) " +
		"WHERE NOT EXISTS (SELECT 1 FROM groups x WHERE lower(x.name) = lower(g.name)) ON CONFLICT DO NOTHING"
	groupSyncAdd = "INSERT INTO group_members (id, group_id, user_id, source) " +
		"SELECT m.id, x.id, $1, $2 FROM unnest($3::uuid[], $4::text[]) AS m(id, name) " +
		"JOIN groups x ON lower(x.name) = m.name ON CONFLICT (group_id, user_id) DO NOTHING"
	groupSyncRemove = "DELETE FROM group_members m USING groups x WHERE m.group_id = x.id AND m.user_id = $1 " +
		"AND m.source = $2 AND NOT (lower(x.name) = ANY($3::text[]))"
	groupExists = "SELECT EXISTS (SELECT 1 FROM groups WHERE id = $1)"
)

func (s *GroupStore) Insert(ctx context.Context, g domain.NewGroup) (domain.Group, error) {
	group, err := scanGroup(s.pool.QueryRow(ctx, groupInsert, g.ID, g.Name))
	return group, dbErr(err)
}

func (s *GroupStore) List(ctx context.Context, page domain.PageRequest) (domain.Page[domain.Group], error) {
	rows, err := s.pool.Query(ctx, groupList, int64(page.Limit), int64(page.Offset))
	if err != nil {
		return domain.Page[domain.Group]{}, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Group, error) { return scanGroup(r) })
	if err != nil {
		return domain.Page[domain.Group]{}, dbErr(err)
	}
	var total int64
	if err := s.pool.QueryRow(ctx, groupCount).Scan(&total); err != nil {
		return domain.Page[domain.Group]{}, dbErr(err)
	}
	return domain.Page[domain.Group]{Items: items, Total: uint64(max(total, 0)), Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *GroupStore) Find(ctx context.Context, id uuid.UUID) (*domain.GroupDetails, error) {
	group, err := scanGroup(s.pool.QueryRow(ctx, groupFind, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	rows, err := s.pool.Query(ctx, groupMembers, id)
	if err != nil {
		return nil, dbErr(err)
	}
	members, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Member, error) {
		var m domain.Member
		var st, source string
		if err := r.Scan(&m.User.ID, &m.User.Email, &m.User.DisplayName, &st, &source); err != nil {
			return m, err
		}
		var err error
		m.User.Status, err = status(st)
		m.Source = domain.MembershipSource(source)
		return m, err
	})
	if err != nil {
		return nil, dbErr(err)
	}
	return &domain.GroupDetails{Group: group, Members: members}, nil
}

func (s *GroupStore) Rename(ctx context.Context, id uuid.UUID, name string) (domain.Group, error) {
	group, err := scanGroup(s.pool.QueryRow(ctx, groupRename, id, name))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Group{}, domain.ErrNotFound
	}
	return group, dbErr(err)
}

func (s *GroupStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, groupDelete, id)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *GroupStore) AddMember(ctx context.Context, id, groupID, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, groupAddMember, id, groupID, userID)
	return dbErr(err)
}

func (s *GroupStore) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, groupRemoveMember, groupID, userID)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	var source string
	switch err := s.pool.QueryRow(ctx, groupMemberSource, groupID, userID).Scan(&source); {
	case err == nil:
		return domain.ConflictMembershipManaged
	case !errors.Is(err, pgx.ErrNoRows):
		return dbErr(err)
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, groupExists, groupID).Scan(&exists); err != nil {
		return dbErr(err)
	}
	if !exists {
		return domain.ErrNotFound
	}
	return nil
}

func (s *GroupStore) SyncManaged(ctx context.Context, userID uuid.UUID, source domain.MembershipSource, groups []string) error {
	ids := make([]uuid.UUID, len(groups))
	memberIDs := make([]uuid.UUID, len(groups))
	lower := make([]string, len(groups))
	for i, g := range groups {
		ids[i], memberIDs[i] = uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
		lower[i] = strings.ToLower(g)
	}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if len(groups) > 0 {
			if _, err := tx.Exec(ctx, groupEnsure, ids, groups); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, groupSyncAdd, userID, string(source), memberIDs, lower); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, groupSyncRemove, userID, string(source), lower)
		return err
	})
	return dbErr(err)
}
