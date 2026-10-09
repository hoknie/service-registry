package access

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/access"
)

type UserStore struct{ pool *pgxpool.Pool }

func NewUserStore(pool *pgxpool.Pool) *UserStore { return &UserStore{pool: pool} }

var (
	userInsert = "INSERT INTO users AS u (id, email, display_name, password_hash, is_superadmin) " +
		"VALUES ($1, $2, $3, NULLIF($4, ''), $5) RETURNING " + userColumns
	userInsertIfNone = "INSERT INTO users (id, email, display_name, password_hash, is_superadmin) " +
		"SELECT $1, $2, $3, NULLIF($4, ''), $5 WHERE NOT EXISTS (SELECT 1 FROM users)"
	userFind               = "SELECT " + userColumns + " FROM users u WHERE u.id = $1"
	userCredentialsByEmail = "SELECT " + userColumns + ", COALESCE(u.password_hash, '') FROM users u WHERE lower(u.email) = lower($1)"
	userCredentialsByID    = "SELECT " + userColumns + ", COALESCE(u.password_hash, '') FROM users u WHERE u.id = $1"
	userList               = "SELECT " + userColumns + " FROM users u ORDER BY u.created_at, u.id LIMIT $1 OFFSET $2"
	userCount              = "SELECT count(*) FROM users"
	userLockSuperadmins    = "SELECT id FROM users WHERE is_superadmin AND status = 'active' ORDER BY id FOR UPDATE"
	userLockOne            = "SELECT id FROM users WHERE id = $1 FOR UPDATE"
	userUpdate             = "UPDATE users AS u SET display_name = COALESCE($2, u.display_name), " +
		"status = COALESCE($3, u.status), is_superadmin = COALESCE($4, u.is_superadmin), " +
		"updated_at = now() WHERE u.id = $1 RETURNING " + userColumns
	userSetPassword = "UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1"
)

func (s *UserStore) Insert(ctx context.Context, u domain.NewUser) (domain.User, error) {
	user, err := scanUser(s.pool.QueryRow(ctx, userInsert, u.ID, u.Email, u.DisplayName, u.PasswordHash, u.IsSuperadmin))
	return user, dbErr(err)
}

func (s *UserStore) InsertIfNone(ctx context.Context, u domain.NewUser) (bool, error) {
	tag, err := s.pool.Exec(ctx, userInsertIfNone, u.ID, u.Email, u.DisplayName, u.PasswordHash, u.IsSuperadmin)
	if err != nil {
		if mapped := dbErr(err); errors.Is(mapped, domain.ConflictEmailTaken) {
			return false, nil
		} else {
			return false, mapped
		}
	}
	return tag.RowsAffected() == 1, nil
}

func (s *UserStore) Find(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, err := scanUser(s.pool.QueryRow(ctx, userFind, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &user, nil
}

func (s *UserStore) credentials(ctx context.Context, query string, arg any) (*domain.UserCredentials, error) {
	var c domain.UserCredentials
	user, err := scanUser(s.pool.QueryRow(ctx, query, arg), &c.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	c.User = user
	return &c, nil
}

func (s *UserStore) FindCredentialsByEmail(ctx context.Context, email string) (*domain.UserCredentials, error) {
	return s.credentials(ctx, userCredentialsByEmail, email)
}

func (s *UserStore) FindCredentials(ctx context.Context, id uuid.UUID) (*domain.UserCredentials, error) {
	return s.credentials(ctx, userCredentialsByID, id)
}

func (s *UserStore) List(ctx context.Context, page domain.PageRequest) (domain.Page[domain.User], error) {
	rows, err := s.pool.Query(ctx, userList, int64(page.Limit), int64(page.Offset))
	if err != nil {
		return domain.Page[domain.User]{}, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.User, error) { return scanUser(r) })
	if err != nil {
		return domain.Page[domain.User]{}, dbErr(err)
	}
	var total int64
	if err := s.pool.QueryRow(ctx, userCount).Scan(&total); err != nil {
		return domain.Page[domain.User]{}, dbErr(err)
	}
	return domain.Page[domain.User]{Items: items, Total: uint64(max(total, 0)), Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *UserStore) Update(ctx context.Context, id uuid.UUID, ch domain.UserChanges) (domain.User, error) {
	var user domain.User
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		mayDemote := (ch.IsSuperadmin != nil && !*ch.IsSuperadmin) ||
			(ch.Status != nil && *ch.Status == domain.StatusDisabled)
		if mayDemote {
			rows, err := tx.Query(ctx, userLockSuperadmins)
			if err != nil {
				return err
			}
			admins, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
			if err != nil {
				return err
			}
			if len(admins) == 1 && slices.Contains(admins, id) {
				return domain.ConflictLastSuperadmin
			}
		}
		var locked uuid.UUID
		if err := tx.QueryRow(ctx, userLockOne, id).Scan(&locked); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		var status *string
		if ch.Status != nil {
			s := string(*ch.Status)
			status = &s
		}
		var err error
		user, err = scanUser(tx.QueryRow(ctx, userUpdate, id, ch.DisplayName, status, ch.IsSuperadmin))
		if err != nil {
			return err
		}
		switch {
		case ch.Status != nil && *ch.Status == domain.StatusDisabled:
			_, err = tx.Exec(ctx, tokenRevokeForUser, id)
		case ch.IsSuperadmin != nil && !*ch.IsSuperadmin:
			_, err = tx.Exec(ctx, tokenRevokeUnexpiringOfUser, id)
		}
		return err
	})
	return user, dbErr(err)
}

func (s *UserStore) SetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	tag, err := s.pool.Exec(ctx, userSetPassword, id, hash)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *UserStore) ResetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, userSetPassword, id, hash)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		_, err = tx.Exec(ctx, tokenRevokeForUser, id)
		return err
	})
	return dbErr(err)
}
