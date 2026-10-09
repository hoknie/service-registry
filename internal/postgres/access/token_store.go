package access

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "svc-registry/internal/access"
)

type TokenStore struct{ pool *pgxpool.Pool }

func NewTokenStore(pool *pgxpool.Pool) *TokenStore { return &TokenStore{pool: pool} }

var (
	tokenInsert = "WITH t AS (INSERT INTO personal_access_tokens (id, user_id, name, prefix, token_hash, scopes, expires_at) " +
		"VALUES ($1, $2, $3, $4, $5, $6, now() + make_interval(days => $7::int)) RETURNING *) " +
		"SELECT " + tokenColumns + " FROM t JOIN users u ON u.id = t.user_id"
	tokenListForUser = "SELECT " + tokenColumns + " FROM personal_access_tokens t JOIN users u ON u.id = t.user_id " +
		"WHERE t.user_id = $1 ORDER BY t.created_at DESC, t.id DESC"
	tokenList = "SELECT " + tokenColumns + " FROM personal_access_tokens t JOIN users u ON u.id = t.user_id " +
		"WHERE t.prefix LIKE $1 ORDER BY t.created_at DESC, t.id DESC LIMIT $2 OFFSET $3"
	tokenCount  = "SELECT count(*) FROM personal_access_tokens t WHERE t.prefix LIKE $1"
	tokenRevoke = "UPDATE personal_access_tokens SET revoked_at = COALESCE(revoked_at, now()) " +
		"WHERE id = $1 AND ($2::uuid IS NULL OR user_id = $2)"
	tokenFindValid = "SELECT t.id, t.scopes, (t.last_used_at IS NULL OR t.last_used_at < now() - interval '60 seconds') AS stale, " +
		userColumns + " FROM personal_access_tokens t JOIN users u ON u.id = t.user_id" +
		" WHERE t.token_hash = $1 AND t.revoked_at IS NULL AND (t.expires_at IS NULL OR t.expires_at > now())" +
		" AND u.status = 'active'"
	tokenTouch                  = "UPDATE personal_access_tokens SET last_used_at = now() WHERE id = $1"
	tokenRevokeForUser          = "UPDATE personal_access_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL"
	tokenRevokeUnexpiringOfUser = tokenRevokeForUser + " AND expires_at IS NULL"
)

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

func (s *TokenStore) Insert(ctx context.Context, nt domain.NewToken) (domain.Token, error) {
	var days *int32
	if nt.ExpiresInDays != nil {
		d := int32(*nt.ExpiresInDays)
		days = &d
	}
	token, err := scanToken(s.pool.QueryRow(ctx, tokenInsert, nt.ID, nt.UserID, nt.Name, nt.Prefix, nt.Hash[:], nt.Scopes.Strings(), days))
	return token, dbErr(err)
}

func (s *TokenStore) ListForUser(ctx context.Context, userID uuid.UUID) ([]domain.Token, error) {
	rows, err := s.pool.Query(ctx, tokenListForUser, userID)
	if err != nil {
		return nil, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Token, error) { return scanToken(r) })
	return items, dbErr(err)
}

func (s *TokenStore) List(ctx context.Context, f domain.TokenFilter) (domain.Page[domain.Token], error) {
	pattern := likePrefix(f.Prefix)
	rows, err := s.pool.Query(ctx, tokenList, pattern, int64(f.Page.Limit), int64(f.Page.Offset))
	if err != nil {
		return domain.Page[domain.Token]{}, dbErr(err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.Token, error) { return scanToken(r) })
	if err != nil {
		return domain.Page[domain.Token]{}, dbErr(err)
	}
	var total int64
	if err := s.pool.QueryRow(ctx, tokenCount, pattern).Scan(&total); err != nil {
		return domain.Page[domain.Token]{}, dbErr(err)
	}
	return domain.Page[domain.Token]{Items: items, Total: uint64(max(total, 0)), Limit: f.Page.Limit, Offset: f.Page.Offset}, nil
}

func (s *TokenStore) Revoke(ctx context.Context, id uuid.UUID, owner *uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, tokenRevoke, id, owner)
	if err != nil {
		return dbErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *TokenStore) FindValid(ctx context.Context, tokenHash [32]byte) (*domain.TokenUser, error) {
	var found domain.TokenUser
	var scopes []string
	var stale bool
	user, err := scanUserAfter(s.pool.QueryRow(ctx, tokenFindValid, tokenHash[:]), &found.TokenID, &scopes, &stale)
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
		if _, err := s.pool.Exec(ctx, tokenTouch, found.TokenID); err != nil {
			return nil, dbErr(err)
		}
	}
	return &found, nil
}
