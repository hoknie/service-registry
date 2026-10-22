package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type IdentityRepository struct{ db *pgtx.DB }

func NewIdentityRepository(db *pgtx.DB) *IdentityRepository { return &IdentityRepository{db: db} }

func insertIdentity(ctx context.Context, db pgtx.Querier, n accesstypes.NewIdentity, userID uuid.UUID) (accesstypes.Identity, error) {
	return scanIdentity(db.QueryRow(ctx, `
		INSERT INTO user_identities AS i (id, user_id, provider, subject, email)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING i.id, i.user_id, i.provider, i.subject, i.email, rfc3339(i.created_at),
			rfc3339(i.last_login_at)`,
		n.ID, userID, n.Provider, n.Subject, n.Email))
}

func scanIdentity(row pgx.Row, extra ...any) (accesstypes.Identity, error) {
	var i accesstypes.Identity
	err := row.Scan(append([]any{&i.ID, &i.UserID, &i.Provider, &i.Subject, &i.Email, &i.CreatedAt, &i.LastLoginAt}, extra...)...)
	return i, err
}

func (s *IdentityRepository) Find(ctx context.Context, provider, subject string) (*accesstypes.Identity, *accesstypes.User, error) {
	var u accesstypes.User
	var st string
	i, err := scanIdentity(s.db.From(ctx).QueryRow(ctx, `
		SELECT i.id, i.user_id, i.provider, i.subject, i.email, rfc3339(i.created_at),
			rfc3339(i.last_login_at), u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service,
			u.password_hash IS NOT NULL AS has_password, rfc3339(u.created_at) AS created_at,
			rfc3339(u.updated_at) AS updated_at
		FROM user_identities i
		JOIN users u ON u.id = i.user_id
		WHERE i.provider = $1
			AND i.subject = $2`, provider, subject),
		&u.ID, &u.Email, &u.DisplayName, &st, &u.IsSuperadmin, &u.IsService, &u.HasPassword, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, postgres.Translate(err, constraints)
	}
	if u.Status, err = status(st); err != nil {
		return nil, nil, err
	}
	return &i, &u, nil
}

func (s *IdentityRepository) Insert(ctx context.Context, n accesstypes.NewIdentity) (accesstypes.Identity, error) {
	i, err := insertIdentity(ctx, s.db.From(ctx), n, n.UserID)
	return i, postgres.Translate(err, constraints)
}

func (s *IdentityRepository) Touch(ctx context.Context, id uuid.UUID, email *string) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE user_identities
		SET last_login_at = now(), email = COALESCE($2, email)
		WHERE id = $1`, id, email)
	return postgres.Translate(err, constraints)
}

func (s *IdentityRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]accesstypes.Identity, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT i.id, i.user_id, i.provider, i.subject, i.email, rfc3339(i.created_at),
			rfc3339(i.last_login_at)
		FROM user_identities i
		WHERE i.user_id = $1
		ORDER BY i.created_at, i.id`, userID)
	if err != nil {
		return nil, postgres.Translate(err, constraints)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (accesstypes.Identity, error) { return scanIdentity(r) })
	return items, postgres.Translate(err, constraints)
}

func (s *IdentityRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		var hasPassword bool
		if err := tx.QueryRow(ctx, `
			SELECT password_hash IS NOT NULL
			FROM users
			WHERE id = $1
			FOR UPDATE`, userID).Scan(&hasPassword); errors.Is(err, pgx.ErrNoRows) {
			return apperr.ErrNotFound
		} else if err != nil {
			return err
		}
		var count int64
		if err := tx.QueryRow(ctx, `
			SELECT count(*)
			FROM user_identities
			WHERE user_id = $1`, userID).Scan(&count); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			DELETE FROM user_identities
			WHERE id = $1
				AND user_id = $2`, id, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.ErrNotFound
		}
		if !hasPassword && count <= 1 {
			return accesstypes.ConflictLastLoginMethod
		}
		return nil
	})
	return postgres.Translate(err, constraints)
}

func (s *IdentityRepository) CreateUser(ctx context.Context, u accesstypes.NewUser, n accesstypes.NewIdentity) (accesstypes.User, error) {
	var user accesstypes.User
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		var err error
		u.PasswordHash = ""
		if user, err = insertUser(ctx, tx, u); err != nil {
			return err
		}
		_, err = insertIdentity(ctx, tx, n, user.ID)
		return err
	})
	return user, postgres.Translate(err, constraints)
}
