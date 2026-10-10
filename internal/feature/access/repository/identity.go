package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/postgres"
)

type Identities struct{ db *postgres.DB }

func NewIdentities(db *postgres.DB) *Identities { return &Identities{db: db} }

func insertIdentity(ctx context.Context, db postgres.Querier, n access.NewIdentity, userID uuid.UUID) (access.Identity, error) {
	return scanIdentity(db.QueryRow(ctx, `
		INSERT INTO user_identities AS i (id, user_id, provider, subject, email)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING i.id, i.user_id, i.provider, i.subject, i.email, rfc3339(i.created_at),
			rfc3339(i.last_login_at)`,
		n.ID, userID, n.Provider, n.Subject, n.Email))
}

func scanIdentity(row pgx.Row, extra ...any) (access.Identity, error) {
	var i access.Identity
	err := row.Scan(append([]any{&i.ID, &i.UserID, &i.Provider, &i.Subject, &i.Email, &i.CreatedAt, &i.LastLoginAt}, extra...)...)
	return i, err
}

func (s *Identities) Find(ctx context.Context, provider, subject string) (*access.Identity, *access.User, error) {
	var u access.User
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
		return nil, nil, dbErr(err)
	}
	if u.Status, err = status(st); err != nil {
		return nil, nil, err
	}
	return &i, &u, nil
}

func (s *Identities) Insert(ctx context.Context, n access.NewIdentity) (access.Identity, error) {
	i, err := insertIdentity(ctx, s.db.From(ctx), n, n.UserID)
	return i, dbErr(err)
}

func (s *Identities) Touch(ctx context.Context, id uuid.UUID, email *string) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		UPDATE user_identities
		SET last_login_at = now(), email = COALESCE($2, email)
		WHERE id = $1`, id, email)
	return dbErr(err)
}

func (s *Identities) ListForUser(ctx context.Context, userID uuid.UUID) ([]access.Identity, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT i.id, i.user_id, i.provider, i.subject, i.email, rfc3339(i.created_at),
			rfc3339(i.last_login_at)
		FROM user_identities i
		WHERE i.user_id = $1
		ORDER BY i.created_at, i.id`, userID)
	if err != nil {
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (access.Identity, error) { return scanIdentity(r) })
	return items, dbErr(err)
}

func (s *Identities) Delete(ctx context.Context, userID, id uuid.UUID) error {
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		tx := s.db.From(ctx)
		var hasPassword bool
		if err := tx.QueryRow(ctx, `
			SELECT password_hash IS NOT NULL
			FROM users
			WHERE id = $1
			FOR UPDATE`, userID).Scan(&hasPassword); errors.Is(err, pgx.ErrNoRows) {
			return access.ErrNotFound
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
			return access.ErrNotFound
		}
		if !hasPassword && count <= 1 {
			return access.ConflictLastLoginMethod
		}
		return nil
	})
	return dbErr(err)
}

func (s *Identities) CreateUser(ctx context.Context, u access.NewUser, n access.NewIdentity) (access.User, error) {
	var user access.User
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
	return user, dbErr(err)
}
