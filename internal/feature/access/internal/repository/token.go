package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/platform/postgres"
)

type Tokens struct{ db *postgres.DB }

func NewTokens(db *postgres.DB) *Tokens { return &Tokens{db: db} }

func revokeUserTokens(ctx context.Context, db postgres.Querier, userID uuid.UUID) error {
	_, err := db.Exec(ctx, `
		UPDATE personal_access_tokens
		SET revoked_at = now()
		WHERE user_id = $1
			AND revoked_at IS NULL`, userID)
	return err
}

func likePrefix(prefix string) string {
	out := make([]rune, 0, len(prefix)+1)
	for _, r := range prefix {
		if r == '%' || r == '_' || r == '\\' {
			out = append(out, '\\')
		}
		out = append(out, r)
	}
	return string(out) + "%"
}

func (s *Tokens) Insert(ctx context.Context, nt access.NewToken) (access.Token, error) {
	var days *int32
	if nt.ExpiresInDays != nil {
		d := int32(*nt.ExpiresInDays)
		days = &d
	}
	token, err := scanToken(s.db.From(ctx).QueryRow(ctx, `
		WITH t AS (
			INSERT INTO personal_access_tokens (id, user_id, name, prefix, token_hash, scopes, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, now() + make_interval(days => $7::int))
			RETURNING *
		)
		SELECT t.id, t.user_id, u.email, t.name, t.prefix, t.scopes, rfc3339(t.created_at) AS created_at,
			rfc3339(t.expires_at) AS expires_at, rfc3339(t.last_used_at) AS last_used_at,
			rfc3339(t.revoked_at) AS revoked_at,
			CASE
				WHEN t.revoked_at IS NOT NULL THEN 'revoked'
				WHEN t.expires_at <= now() THEN 'expired'
				ELSE 'active' END AS status
		FROM t
		JOIN users u ON u.id = t.user_id`, nt.ID, nt.UserID, nt.Name, nt.Prefix, nt.Hash[:], nt.Scopes.Strings(), days))
	return token, dbErr(err)
}

func (s *Tokens) ListForUser(ctx context.Context, userID uuid.UUID) ([]access.Token, error) {
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT t.id, t.user_id, u.email, t.name, t.prefix, t.scopes, rfc3339(t.created_at) AS created_at,
			rfc3339(t.expires_at) AS expires_at, rfc3339(t.last_used_at) AS last_used_at,
			rfc3339(t.revoked_at) AS revoked_at,
			CASE
				WHEN t.revoked_at IS NOT NULL THEN 'revoked'
				WHEN t.expires_at <= now() THEN 'expired'
				ELSE 'active' END AS status
		FROM personal_access_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.user_id = $1
		ORDER BY t.created_at DESC, t.id DESC`, userID)
	if err != nil {
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (access.Token, error) { return scanToken(r) })
	return items, dbErr(err)
}

func (s *Tokens) List(ctx context.Context, f access.TokenFilter) (access.Page[access.Token], error) {
	pattern := likePrefix(f.Prefix)
	rows, err := s.db.From(ctx).Query(ctx, `
		SELECT t.id, t.user_id, u.email, t.name, t.prefix, t.scopes, rfc3339(t.created_at) AS created_at,
			rfc3339(t.expires_at) AS expires_at, rfc3339(t.last_used_at) AS last_used_at,
			rfc3339(t.revoked_at) AS revoked_at,
			CASE
				WHEN t.revoked_at IS NOT NULL THEN 'revoked'
				WHEN t.expires_at <= now() THEN 'expired'
				ELSE 'active' END AS status
		FROM personal_access_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.prefix LIKE $1
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT $2
		OFFSET $3`, pattern, int64(f.Page.Limit), int64(f.Page.Offset))
	if err != nil {
		return access.Page[access.Token]{}, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (access.Token, error) { return scanToken(r) })
	if err != nil {
		return access.Page[access.Token]{}, dbErr(err)
	}
	var total int64
	if err := s.db.From(ctx).QueryRow(ctx, `
		SELECT count(*)
		FROM personal_access_tokens t
		WHERE t.prefix LIKE $1`, pattern).Scan(&total); err != nil {
		return access.Page[access.Token]{}, dbErr(err)
	}
	return access.Page[access.Token]{Items: items, Total: uint64(max(total, 0)), Limit: f.Page.Limit, Offset: f.Page.Offset}, nil
}

func (s *Tokens) Revoke(ctx context.Context, id uuid.UUID, owner *uuid.UUID) error {
	tag, err := s.db.From(ctx).Exec(ctx, `
		UPDATE personal_access_tokens
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE id = $1
			AND ($2::uuid IS NULL OR user_id = $2)`, id, owner)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return access.ErrNotFound
	}
	return nil
}

func (s *Tokens) FindValid(ctx context.Context, tokenHash [32]byte) (*access.TokenUser, error) {
	var found access.TokenUser
	var scopes []string
	var stale bool
	user, err := scanUserAfter(s.db.From(ctx).QueryRow(ctx, `
		SELECT t.id, t.scopes,
			(t.last_used_at IS NULL OR t.last_used_at < now() - interval '60 seconds') AS stale, u.id, u.email,
			u.display_name, u.status, u.is_superadmin, u.is_service, u.password_hash IS NOT NULL AS has_password,
			rfc3339(u.created_at) AS created_at, rfc3339(u.updated_at) AS updated_at
		FROM personal_access_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1
			AND t.revoked_at IS NULL
			AND (t.expires_at IS NULL OR t.expires_at > now())
			AND u.status = 'active'`, tokenHash[:]), &found.TokenID, &scopes, &stale)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, dbErr(err)
	}
	if found.Scopes, err = parseScopes(scopes); err != nil {
		return nil, err
	}
	found.User = user
	if stale {
		if _, err := s.db.From(ctx).Exec(ctx, `
			UPDATE personal_access_tokens
			SET last_used_at = now()
			WHERE id = $1`, found.TokenID); err != nil {
			return nil, dbErr(err)
		}
	}
	return &found, nil
}
