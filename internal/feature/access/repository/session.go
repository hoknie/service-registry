package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/postgres"
)

type Sessions struct{ db *postgres.DB }

func NewSessions(db *postgres.DB) *Sessions { return &Sessions{db: db} }

func secs(n uint64) float64 { return float64(n) }

func (s *Sessions) Create(ctx context.Context, ns access.NewSession) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, expires_at, method)
		VALUES ($1, $2, $3, now() + make_interval(secs => $4), COALESCE(NULLIF($5, ''), 'password'))`,
		ns.ID, ns.UserID, ns.TokenHash[:], secs(ns.AbsoluteTimeoutSecs), ns.Method)
	return dbErr(err)
}

func (s *Sessions) FindValid(ctx context.Context, tokenHash [32]byte, idleTimeoutSecs uint64) (*access.SessionUser, error) {
	var found access.SessionUser
	var stale bool
	row := s.db.From(ctx).QueryRow(ctx, `
		SELECT s.id AS session_id, s.last_seen_at < now() - interval '60 seconds' AS stale, s.method, u.id,
			u.email, u.display_name, u.status, u.is_superadmin, u.is_service, u.password_hash IS NOT NULL AS has_password,
			rfc3339(u.created_at) AS created_at, rfc3339(u.updated_at) AS updated_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1
			AND s.expires_at > now()
			AND s.last_seen_at > now() - make_interval(secs => $2)
			AND u.status = 'active'`, tokenHash[:], secs(idleTimeoutSecs))
	var u access.User
	var st string
	err := row.Scan(&found.SessionID, &stale, &found.Method, &u.ID, &u.Email, &u.DisplayName, &st, &u.IsSuperadmin, &u.IsService, &u.HasPassword, &u.CreatedAt, &u.UpdatedAt)
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
		if _, err := s.db.From(ctx).Exec(ctx, `
			UPDATE sessions
			SET last_seen_at = now()
			WHERE id = $1`, found.SessionID); err != nil {
			return nil, dbErr(err)
		}
	}
	return &found, nil
}

func (s *Sessions) DeleteByToken(ctx context.Context, tokenHash [32]byte) error {
	_, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM sessions
		WHERE token_hash = $1`, tokenHash[:])
	return dbErr(err)
}

func (s *Sessions) DeleteForUser(ctx context.Context, userID uuid.UUID, keep *uuid.UUID) (uint64, error) {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM sessions
		WHERE user_id = $1
			AND ($2::uuid IS NULL OR id <> $2)`, userID, keep)
	if err != nil {
		return 0, dbErr(err)
	}
	return uint64(tag.RowsAffected()), nil
}

func (s *Sessions) DeleteExpiredForUser(ctx context.Context, userID uuid.UUID, idleTimeoutSecs uint64) (uint64, error) {
	tag, err := s.db.From(ctx).Exec(ctx, `
		DELETE FROM sessions
		WHERE user_id = $1
			AND (expires_at <= now() OR last_seen_at <= now() - make_interval(secs => $2))`, userID, secs(idleTimeoutSecs))
	if err != nil {
		return 0, dbErr(err)
	}
	return uint64(tag.RowsAffected()), nil
}
