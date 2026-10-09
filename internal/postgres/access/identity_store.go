package access

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/access"
	"svc-registry/internal/postgres"
)

type IdentityStore struct{ pool *pgxpool.Pool }

func NewIdentityStore(pool *pgxpool.Pool) *IdentityStore { return &IdentityStore{pool: pool} }

var (
	identityColumns = "i.id, i.user_id, i.provider, i.subject, i.email, " + postgres.RFC3339("i.created_at") + ", " +
		postgres.RFC3339("i.last_login_at")
	identityFind = "SELECT " + identityColumns + ", " + userColumns +
		" FROM user_identities i JOIN users u ON u.id = i.user_id WHERE i.provider = $1 AND i.subject = $2"
	identityInsert = "INSERT INTO user_identities AS i (id, user_id, provider, subject, email) VALUES ($1, $2, $3, $4, $5) " +
		"RETURNING " + identityColumns
	identityTouch  = "UPDATE user_identities SET last_login_at = now(), email = COALESCE($2, email) WHERE id = $1"
	identityList   = "SELECT " + identityColumns + " FROM user_identities i WHERE i.user_id = $1 ORDER BY i.created_at, i.id"
	identityLock   = "SELECT password_hash IS NOT NULL FROM users WHERE id = $1 FOR UPDATE"
	identityCount  = "SELECT count(*) FROM user_identities WHERE user_id = $1"
	identityDelete = "DELETE FROM user_identities WHERE id = $1 AND user_id = $2"
)

func scanIdentity(row pgx.Row, extra ...any) (domain.Identity, error) {
	var i domain.Identity
	err := row.Scan(append([]any{&i.ID, &i.UserID, &i.Provider, &i.Subject, &i.Email, &i.CreatedAt, &i.LastLoginAt}, extra...)...)
	return i, err
}

func (s *IdentityStore) Find(ctx context.Context, provider, subject string) (*domain.Identity, *domain.User, error) {
	var u domain.User
	var st string
	i, err := scanIdentity(s.pool.QueryRow(ctx, identityFind, provider, subject),
		&u.ID, &u.Email, &u.DisplayName, &st, &u.IsSuperadmin, &u.HasPassword, &u.CreatedAt, &u.UpdatedAt)
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

func (s *IdentityStore) Insert(ctx context.Context, n domain.NewIdentity) (domain.Identity, error) {
	i, err := scanIdentity(s.pool.QueryRow(ctx, identityInsert, n.ID, n.UserID, n.Provider, n.Subject, n.Email))
	return i, dbErr(err)
}

func (s *IdentityStore) Touch(ctx context.Context, id uuid.UUID, email *string) error {
	_, err := s.pool.Exec(ctx, identityTouch, id, email)
	return dbErr(err)
}

func (s *IdentityStore) ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.Identity, error) {
	rows, err := s.pool.Query(ctx, identityList, userID)
	if err != nil {
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Identity, error) { return scanIdentity(r) })
	return items, dbErr(err)
}

func (s *IdentityStore) Delete(ctx context.Context, userID, id uuid.UUID) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var hasPassword bool
		if err := tx.QueryRow(ctx, identityLock, userID).Scan(&hasPassword); errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		} else if err != nil {
			return err
		}
		var count int64
		if err := tx.QueryRow(ctx, identityCount, userID).Scan(&count); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, identityDelete, id, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		if !hasPassword && count <= 1 {
			return domain.ConflictLastLoginMethod
		}
		return nil
	})
	return dbErr(err)
}

func (s *IdentityStore) CreateUser(ctx context.Context, u domain.NewUser, n domain.NewIdentity) (domain.User, error) {
	var user domain.User
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if user, err = scanUser(tx.QueryRow(ctx, userInsert, u.ID, u.Email, u.DisplayName, "", u.IsSuperadmin)); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, identityInsert, n.ID, user.ID, n.Provider, n.Subject, n.Email)
		return err
	})
	return user, dbErr(err)
}
