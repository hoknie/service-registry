package access

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/access"
)

type SessionStore struct{ pool *pgxpool.Pool }

func NewSessionStore(pool *pgxpool.Pool) *SessionStore { return &SessionStore{pool: pool} }

func secs(n uint64) float64 { return float64(n) }

func (s *SessionStore) Create(ctx context.Context, ns domain.NewSession) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, expires_at, method)
		VALUES ($1, $2, $3, now() + make_interval(secs => $4), COALESCE(NULLIF($5, ''), 'password'))`,
		ns.ID, ns.UserID, ns.TokenHash[:], secs(ns.AbsoluteTimeoutSecs), ns.Method)
	return dbErr(err)
}

func (s *SessionStore) FindValid(ctx context.Context, tokenHash [32]byte, idleTimeoutSecs uint64) (*domain.SessionUser, error) {
	var found domain.SessionUser
	var stale bool
	row := s.pool.QueryRow(ctx, `
		SELECT s.id AS session_id, s.last_seen_at < now() - interval '60 seconds' AS stale, s.method, u.id,
			u.email, u.display_name, u.status, u.is_superadmin, u.password_hash IS NOT NULL AS has_password,
			rfc3339(u.created_at) AS created_at, rfc3339(u.updated_at) AS updated_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1
			AND s.expires_at > now()
			AND s.last_seen_at > now() - make_interval(secs => $2)
			AND u.status = 'active'`, tokenHash[:], secs(idleTimeoutSecs))
	var u domain.User
	var st string
	err := row.Scan(&found.SessionID, &stale, &found.Method, &u.ID, &u.Email, &u.DisplayName, &st, &u.IsSuperadmin, &u.HasPassword, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	if u.Status, err = status(st); err != nil {
		return nil, err
	}
	found.User = u
	if stale {
		if _, err := s.pool.Exec(ctx, `
			UPDATE sessions
			SET last_seen_at = now()
			WHERE id = $1`, found.SessionID); err != nil {
			return nil, dbErr(err)
		}
	}
	return &found, nil
}

func (s *SessionStore) DeleteByToken(ctx context.Context, tokenHash [32]byte) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM sessions
		WHERE token_hash = $1`, tokenHash[:])
	return dbErr(err)
}

func (s *SessionStore) DeleteForUser(ctx context.Context, userID uuid.UUID, keep *uuid.UUID) (uint64, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM sessions
		WHERE user_id = $1
			AND ($2::uuid IS NULL OR id <> $2)`, userID, keep)
	if err != nil {
		return 0, dbErr(err)
	}
	return uint64(tag.RowsAffected()), nil
}

func (s *SessionStore) DeleteExpiredForUser(ctx context.Context, userID uuid.UUID, idleTimeoutSecs uint64) (uint64, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM sessions
		WHERE user_id = $1
			AND (expires_at <= now() OR last_seen_at <= now() - make_interval(secs => $2))`, userID, secs(idleTimeoutSecs))
	if err != nil {
		return 0, dbErr(err)
	}
	return uint64(tag.RowsAffected()), nil
}
