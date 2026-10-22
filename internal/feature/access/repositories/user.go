package repositories

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type UserRepository struct{ db *pgtx.DB }

func NewUserRepository(db *pgtx.DB) *UserRepository { return &UserRepository{db: db} }

func insertUser(ctx context.Context, db pgtx.Querier, u accesstypes.NewUser) (accesstypes.User, error) {
	return postgres.One(ctx, db, constraints, scanUser, `
		INSERT INTO users AS u (id, email, display_name, password_hash, is_superadmin, is_service)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6)
		RETURNING u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at`, u.ID, u.Email, u.DisplayName, u.PasswordHash, u.IsSuperadmin, u.IsService)
}

func setPassword(ctx context.Context, db pgtx.Querier, id uuid.UUID, hash string) error {
	return postgres.ExecFound(ctx, db, constraints, `
		UPDATE users
		SET password_hash = $2, updated_at = now()
		WHERE id = $1`, id, hash)
}

func (s *UserRepository) Insert(ctx context.Context, u accesstypes.NewUser) (accesstypes.User, error) {
	return insertUser(ctx, s.db.From(ctx), u)
}

func (s *UserRepository) InsertIfNone(ctx context.Context, u accesstypes.NewUser) (bool, error) {
	n, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO users (id, email, display_name, password_hash, is_superadmin)
		SELECT $1, $2, $3, NULLIF($4, ''), $5
		WHERE NOT EXISTS (SELECT 1 FROM users)`, u.ID, u.Email, u.DisplayName, u.PasswordHash, u.IsSuperadmin)
	if errors.Is(err, accesstypes.ConflictEmailTaken) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (s *UserRepository) Find(ctx context.Context, id uuid.UUID) (*accesstypes.User, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanUser, `
		SELECT u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at
		FROM users u
		WHERE u.id = $1`, id)
}

func (s *UserRepository) credentials(ctx context.Context, query string, arg any) (*accesstypes.UserCredentials, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (accesstypes.UserCredentials, error) {
		var c accesstypes.UserCredentials
		var err error
		c.User, err = scanUserWith(r, nil, []any{&c.PasswordHash})
		return c, err
	}, query, arg)
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
	items, err := postgres.List(ctx, s.db.From(ctx), constraints, scanUser, `
		SELECT u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at
		FROM users u
		ORDER BY u.created_at, u.id
		LIMIT $1
		OFFSET $2`, int64(page.Limit), int64(page.Offset))
	if err != nil {
		return accesstypes.Page[accesstypes.User]{}, err
	}
	total, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM users`)
	if err != nil {
		return accesstypes.Page[accesstypes.User]{}, err
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
			admins, err := postgres.List(ctx, tx, constraints, pgx.RowTo[uuid.UUID], `
				SELECT id
				FROM users
				WHERE is_superadmin
					AND status = 'active'
				ORDER BY id
				FOR UPDATE`)
			if err != nil {
				return err
			}
			if len(admins) == 1 && slices.Contains(admins, id) {
				return accesstypes.ConflictLastSuperadmin
			}
		}
		locked, err := postgres.Find(ctx, tx, constraints, pgx.RowTo[uuid.UUID], `
			SELECT id
			FROM users
			WHERE id = $1
			FOR UPDATE`, id)
		if err != nil {
			return err
		}
		if locked == nil {
			return apperr.ErrNotFound
		}
		var status *string
		if ch.Status != nil {
			status = new(string(*ch.Status))
		}
		user, err = postgres.One(ctx, tx, constraints, scanUser, `
			UPDATE users AS u
			SET display_name = COALESCE($2, u.display_name), status = COALESCE($3, u.status),
				is_superadmin = COALESCE($4, u.is_superadmin), is_service = COALESCE($5, u.is_service), updated_at = now()
			WHERE u.id = $1
			RETURNING u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
				u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
				rfc3339(u.updated_at) AS updated_at`, id, ch.DisplayName, status, ch.IsSuperadmin, ch.IsService)
		if err != nil {
			return err
		}
		if ch.IsService != nil && *ch.IsService {
			if _, err := postgres.Exec(ctx, tx, constraints, `
				DELETE FROM sessions
				WHERE user_id = $1`, id); err != nil {
				return err
			}
		}
		switch {
		case ch.Status != nil && *ch.Status == accesstypes.StatusDisabled:
			err = revokeUserTokens(ctx, tx, id)
		case ch.IsSuperadmin != nil && !*ch.IsSuperadmin:
			_, err = postgres.Exec(ctx, tx, constraints, `
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
	return setPassword(ctx, s.db.From(ctx), id, hash)
}

func (s *UserRepository) ResetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		if err := setPassword(ctx, tx, id, hash); err != nil {
			return err
		}
		return revokeUserTokens(ctx, tx, id)
	})
	return postgres.Translate(err, constraints)
}
