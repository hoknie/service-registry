package repository

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/postgres"
)

type Users struct{ db *postgres.DB }

func NewUsers(db *postgres.DB) *Users { return &Users{db: db} }

func insertUser(ctx context.Context, db postgres.Querier, u access.NewUser) (access.User, error) {
	return scanUser(db.QueryRow(ctx, `
		INSERT INTO users AS u (id, email, display_name, password_hash, is_superadmin, is_service)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6)
		RETURNING u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at`, u.ID, u.Email, u.DisplayName, u.PasswordHash, u.IsSuperadmin, u.IsService))
}

func setPassword(ctx context.Context, db postgres.Querier, id uuid.UUID, hash string) (pgconn.CommandTag, error) {
	return db.Exec(ctx, `
		UPDATE users
		SET password_hash = $2, updated_at = now()
		WHERE id = $1`, id, hash)
}

func (s *Users) Insert(ctx context.Context, u access.NewUser) (access.User, error) {
	user, err := insertUser(ctx, s.db.From(ctx), u)
	return user, dbErr(err)
}

func (s *Users) InsertIfNone(ctx context.Context, u access.NewUser) (bool, error) {
	tag, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO users (id, email, display_name, password_hash, is_superadmin)
		SELECT $1, $2, $3, NULLIF($4, ''), $5
		WHERE NOT EXISTS (SELECT 1 FROM users)`, u.ID, u.Email, u.DisplayName, u.PasswordHash, u.IsSuperadmin)
	if err != nil {
		if mapped := dbErr(err); errors.Is(mapped, access.ConflictEmailTaken) {
			return false, nil
		} else {
			return false, mapped
		}
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Users) Find(ctx context.Context, id uuid.UUID) (*access.User, error) {
	user, err := scanUser(s.db.From(ctx).QueryRow(ctx, `
		SELECT u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at
		FROM users u
		WHERE u.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	return &user, nil
}

func (s *Users) credentials(ctx context.Context, query string, arg any) (*access.UserCredentials, error) {
	var c access.UserCredentials
	user, err := scanUser(s.db.From(ctx).QueryRow(ctx, query, arg), &c.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	c.User = user
	return &c, nil
}

func (s *Users) FindCredentialsByEmail(ctx context.Context, email string) (*access.UserCredentials, error) {
	return s.credentials(ctx, `
		SELECT u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at, COALESCE(u.password_hash, '')
		FROM users u
		WHERE lower(u.email) = lower($1)`, email)
}

func (s *Users) FindCredentials(ctx context.Context, id uuid.UUID) (*access.UserCredentials, error) {
	return s.credentials(ctx, `
		SELECT u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at, COALESCE(u.password_hash, '')
		FROM users u
		WHERE u.id = $1`, id)
}

func (s *Users) List(ctx context.Context, page access.PageRequest) (access.Page[access.User], error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at
		FROM users u
		ORDER BY u.created_at, u.id
		LIMIT $1
		OFFSET $2`, int64(page.Limit), int64(page.Offset))
	if err != nil {
		return access.Page[access.User]{}, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (access.User, error) { return scanUser(r) })
	if err != nil {
		return access.Page[access.User]{}, dbErr(err)
	}
	var total int64
	if err := s.db.From(ctx).QueryRow(ctx, `
		SELECT count(*)
		FROM users`).Scan(&total); err != nil {
		return access.Page[access.User]{}, dbErr(err)
	}
	return access.Page[access.User]{Items: items, Total: uint64(max(total, 0)), Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *Users) Update(ctx context.Context, id uuid.UUID, ch access.UserChanges) (access.User, error) {
	var user access.User
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		mayDemote := (ch.IsSuperadmin != nil && !*ch.IsSuperadmin) ||
			(ch.Status != nil && *ch.Status == access.StatusDisabled)
		if mayDemote {
			rows, err := tx.Query(ctx, `
				SELECT id
				FROM users
				WHERE is_superadmin
					AND status = 'active'
				ORDER BY id
				FOR UPDATE`)
			if err != nil {
				return err
			}
			admins, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
			if err != nil {
				return err
			}
			if len(admins) == 1 && slices.Contains(admins, id) {
				return access.ConflictLastSuperadmin
			}
		}
		var locked uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT id
			FROM users
			WHERE id = $1
			FOR UPDATE`, id).Scan(&locked); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return access.ErrNotFound
			}
			return err
		}
		var status *string
		if ch.Status != nil {
			s := string(*ch.Status)
			status = &s
		}
		var err error
		user, err = scanUser(tx.QueryRow(ctx, `
			UPDATE users AS u
			SET display_name = COALESCE($2, u.display_name), status = COALESCE($3, u.status),
				is_superadmin = COALESCE($4, u.is_superadmin), is_service = COALESCE($5, u.is_service), updated_at = now()
			WHERE u.id = $1
			RETURNING u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
				u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
				rfc3339(u.updated_at) AS updated_at`, id, ch.DisplayName, status, ch.IsSuperadmin, ch.IsService))
		if err != nil {
			return err
		}
		if ch.IsService != nil && *ch.IsService {
			if _, err := tx.Exec(ctx, `
				DELETE FROM sessions
				WHERE user_id = $1`, id); err != nil {
				return err
			}
		}
		switch {
		case ch.Status != nil && *ch.Status == access.StatusDisabled:
			err = revokeUserTokens(ctx, tx, id)
		case ch.IsSuperadmin != nil && !*ch.IsSuperadmin:
			_, err = tx.Exec(ctx, `
				UPDATE personal_access_tokens
				SET revoked_at = now()
				WHERE user_id = $1
					AND revoked_at IS NULL
					AND expires_at IS NULL`, id)
		}
		return err
	})
	return user, dbErr(err)
}

func (s *Users) SetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	tag, err := setPassword(ctx, s.db.From(ctx), id, hash)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return access.ErrNotFound
	}
	return nil
}

func (s *Users) ResetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		tag, err := setPassword(ctx, tx, id, hash)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return access.ErrNotFound
		}
		return revokeUserTokens(ctx, tx, id)
	})
	return dbErr(err)
}
