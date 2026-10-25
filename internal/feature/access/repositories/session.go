package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type SessionRepository struct{ db *pgtx.DB }

func NewSessionRepository(db *pgtx.DB) *SessionRepository { return &SessionRepository{db: db} }

func secs(n uint64) float64 { return float64(n) }

func (s *SessionRepository) Create(ctx context.Context, ns accesstypes.NewSession) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO sessions (id, user_id, token_hash, expires_at, method)
		VALUES ($1, $2, $3, now() + make_interval(secs => $4), $5)`,
		ns.ID, ns.UserID, ns.TokenHash[:], secs(ns.AbsoluteTimeoutSecs), ns.Method)
	return err
}

func (s *SessionRepository) FindValid(ctx context.Context, tokenHash [32]byte, idleTimeoutSecs uint64) (*accesstypes.SessionUser, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (accesstypes.SessionUser, error) {
		var f accesstypes.SessionUser
		var err error
		f.User, err = scanUserWith(r, []any{&f.SessionID, &f.Method}, nil)
		return f, err
	}, `
		WITH found AS (
			SELECT s.id AS session_id, s.last_seen_at < now() - interval '60 seconds' AS stale, s.method, u.id,
				u.email, u.display_name, u.status, u.is_superadmin, u.is_service, u.password_hash IS NOT NULL AS has_password,
				rfc3339(u.created_at) AS created_at, rfc3339(u.updated_at) AS updated_at
			FROM sessions s
			JOIN users u ON u.id = s.user_id
			WHERE s.token_hash = $1
				AND s.expires_at > now()
				AND s.last_seen_at > now() - make_interval(secs => $2)
				AND u.status = 'active'
		),
		touched AS (
			UPDATE sessions s
			SET last_seen_at = now()
			FROM found f
			WHERE s.id = f.session_id
				AND f.stale
		)
		SELECT session_id, method, id, email, display_name, status, is_superadmin, is_service, has_password,
			created_at, updated_at
		FROM found`, tokenHash[:], secs(idleTimeoutSecs))
}

func (s *SessionRepository) DeleteByToken(ctx context.Context, tokenHash [32]byte) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM sessions
		WHERE token_hash = $1`, tokenHash[:])
	return err
}

func (s *SessionRepository) DeleteForUser(ctx context.Context, userID uuid.UUID, keep *uuid.UUID) (uint64, error) {
	n, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM sessions
		WHERE user_id = $1
			AND ($2::uuid IS NULL OR id <> $2)`, userID, keep)
	return uint64(n), err
}

func (s *SessionRepository) DeleteExpiredForUser(ctx context.Context, userID uuid.UUID, idleTimeoutSecs uint64) (uint64, error) {
	n, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM sessions
		WHERE user_id = $1
			AND (expires_at <= now() OR last_seen_at <= now() - make_interval(secs => $2))`, userID, secs(idleTimeoutSecs))
	return uint64(n), err
}

func (s *SessionRepository) MetricCounts(ctx context.Context) (map[string]int64, error) {
	return postgres.Pairs[string, int64](ctx, s.db.From(ctx), constraints, `
		SELECT 'users:' || CASE WHEN is_service THEN 'service' ELSE status END, count(*)
		FROM users
		GROUP BY 1
		UNION ALL
		SELECT 'groups', count(*)
		FROM groups
		UNION ALL
		SELECT 'sessions', count(*)
		FROM sessions
		WHERE expires_at > now()
		UNION ALL
		SELECT 'credentials:personal_token', count(*)
		FROM personal_access_tokens
		WHERE revoked_at IS NULL
			AND (expires_at IS NULL OR expires_at > now())
		UNION ALL
		SELECT 'credentials:oauth_grant', count(*)
		FROM oauth_grants
		WHERE revoked_at IS NULL
			AND refresh_expires_at > now()
		UNION ALL
		SELECT 'active:' || w.name, count(DISTINCT u.user_id)
		FROM (VALUES ('1d', 1), ('7d', 7), ('30d', 30)) w (name, days)
		LEFT JOIN (
			SELECT user_id, last_seen_at AS at
			FROM sessions
			UNION ALL
			SELECT user_id, last_used_at
			FROM personal_access_tokens
			WHERE last_used_at IS NOT NULL
		) u ON u.at >= now() - make_interval(days => w.days)
		GROUP BY w.name`)
}
