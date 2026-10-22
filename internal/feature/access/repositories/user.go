package repositories

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
)

type UserRepository struct{ db *postgres.DB }

func NewUserRepository(db *postgres.DB) *UserRepository { return &UserRepository{db: db} }

func insertUser(ctx context.Context, db postgres.Querier, u accesstypes.NewUser) (accesstypes.User, error) {
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

func (s *UserRepository) Insert(ctx context.Context, u accesstypes.NewUser) (accesstypes.User, error) {
	user, err := insertUser(ctx, s.db.From(ctx), u)
	return user, postgres.Translate(err, constraints)
}

func (s *UserRepository) InsertIfNone(ctx context.Context, u accesstypes.NewUser) (bool, error) {
	tag, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO users (id, email, display_name, password_hash, is_superadmin)
		SELECT $1, $2, $3, NULLIF($4, ''), $5
		WHERE NOT EXISTS (SELECT 1 FROM users)`, u.ID, u.Email, u.DisplayName, u.PasswordHash, u.IsSuperadmin)
	if err != nil {
		if mapped := postgres.Translate(err, constraints); errors.Is(mapped, accesstypes.ConflictEmailTaken) {
			return false, nil
		} else {
			return false, mapped
		}
	}
	return tag.RowsAffected() == 1, nil
}

func (s *UserRepository) Find(ctx context.Context, id uuid.UUID) (*accesstypes.User, error) {
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
		return nil, postgres.Translate(err, constraints)
	}
	return &user, nil
}

func (s *UserRepository) credentials(ctx context.Context, query string, arg any) (*accesstypes.UserCredentials, error) {
	var c accesstypes.UserCredentials
	user, err := scanUser(s.db.From(ctx).QueryRow(ctx, query, arg), &c.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	c.User = user
	return &c, nil
}

func (s *UserRepository) FindCredentialsByEmail(ctx context.Context, email string) (*accesstypes.UserCredentials, error) {
	return s.credentials(ctx, `
		SELECT u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at, COALESCE(u.password_hash, '')
		FROM users u
		WHERE lower(u.email) = lower($1)`, email)
}

func (s *UserRepository) FindCredentials(ctx context.Context, id uuid.UUID) (*accesstypes.UserCredentials, error) {
	return s.credentials(ctx, `
		SELECT u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at, COALESCE(u.password_hash, '')
		FROM users u
		WHERE u.id = $1`, id)
}

func (s *UserRepository) List(ctx context.Context, page accesstypes.PageRequest) (accesstypes.Page[accesstypes.User], error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at
		FROM users u
		ORDER BY u.created_at, u.id
		LIMIT $1
		OFFSET $2`, int64(page.Limit), int64(page.Offset))
	if err != nil {
		return accesstypes.Page[accesstypes.User]{}, postgres.Translate(err, constraints)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (accesstypes.User, error) { return scanUser(r) })
	if err != nil {
		return accesstypes.Page[accesstypes.User]{}, postgres.Translate(err, constraints)
	}
	var total int64
	if err := s.db.From(ctx).QueryRow(ctx, `
		SELECT count(*)
		FROM users`).Scan(&total); err != nil {
		return accesstypes.Page[accesstypes.User]{}, postgres.Translate(err, constraints)
	}
	return accesstypes.Page[accesstypes.User]{Items: items, Total: uint64(max(total, 0)), Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *UserRepository) Update(ctx context.Context, id uuid.UUID, ch accesstypes.UserChanges) (accesstypes.User, error) {
	var user accesstypes.User
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		mayDemote := (ch.IsSuperadmin != nil && !*ch.IsSuperadmin) ||
			(ch.Status != nil && *ch.Status == accesstypes.StatusDisabled)
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
				return accesstypes.ConflictLastSuperadmin
			}
		}
		var locked uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT id
			FROM users
			WHERE id = $1
			FOR UPDATE`, id).Scan(&locked); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.ErrNotFound
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
		case ch.Status != nil && *ch.Status == accesstypes.StatusDisabled:
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
	return user, postgres.Translate(err, constraints)
}

func (s *UserRepository) SetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	tag, err := setPassword(ctx, s.db.From(ctx), id, hash)
	if err != nil {
		return postgres.Translate(err, constraints)
	}
	if tag.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

func (s *UserRepository) ResetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		tag, err := setPassword(ctx, tx, id, hash)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		return revokeUserTokens(ctx, tx, id)
	})
	return postgres.Translate(err, constraints)
}
