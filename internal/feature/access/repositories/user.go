package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type UserRepository struct{ db *pgtx.DB }

func NewUserRepository(db *pgtx.DB) *UserRepository { return &UserRepository{db: db} }

func (s *UserRepository) Insert(ctx context.Context, u accesstypes.NewUser) (accesstypes.User, error) {
	return postgres.One(ctx, s.db.From(ctx), constraints, scanUser, `
		INSERT INTO users AS u (id, email, display_name, password_hash, is_superadmin, is_service)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6)
		RETURNING u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at`, u.ID, u.Email, u.DisplayName, u.PasswordHash, u.IsSuperadmin, u.IsService)
}

func (s *UserRepository) InsertIfNone(ctx context.Context, u accesstypes.NewUser) (bool, error) {
	n, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO users (id, email, display_name, password_hash, is_superadmin)
		SELECT $1, $2, $3, NULLIF($4, ''), $5
		WHERE NOT EXISTS (SELECT 1 FROM users)`, u.ID, u.Email, u.DisplayName, u.PasswordHash, u.IsSuperadmin)
	return n == 1, err
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

func (s *UserRepository) LockActiveSuperadmins(ctx context.Context) ([]uuid.UUID, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		SELECT id
		FROM users
		WHERE is_superadmin
			AND status = 'active'
		ORDER BY id
		FOR UPDATE`)
}

func (s *UserRepository) Lock(ctx context.Context, id uuid.UUID) (bool, error) {
	locked, err := postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[uuid.UUID], `
		SELECT id
		FROM users
		WHERE id = $1
		FOR UPDATE`, id)
	return locked != nil, err
}

func (s *UserRepository) LockPassword(ctx context.Context, id uuid.UUID) (*bool, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, pgx.RowTo[bool], `
		SELECT password_hash IS NOT NULL
		FROM users
		WHERE id = $1
		FOR UPDATE`, id)
}

func (s *UserRepository) Update(ctx context.Context, id uuid.UUID, ch accesstypes.UserChanges) (accesstypes.User, error) {
	var status *string
	if ch.Status != nil {
		status = new(string(*ch.Status))
	}
	return postgres.One(ctx, s.db.From(ctx), constraints, scanUser, `
		UPDATE users AS u
		SET display_name = COALESCE($2, u.display_name), status = COALESCE($3, u.status),
			is_superadmin = COALESCE($4, u.is_superadmin), is_service = COALESCE($5, u.is_service), updated_at = now()
		WHERE u.id = $1
		RETURNING u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at`, id, ch.DisplayName, status, ch.IsSuperadmin, ch.IsService)
}

func (s *UserRepository) SetPassword(ctx context.Context, id uuid.UUID, hash string) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE users
		SET password_hash = $2, updated_at = now()
		WHERE id = $1`, id, hash)
}
