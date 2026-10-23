package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type TokenRepository struct{ db *pgtx.DB }

func NewTokenRepository(db *pgtx.DB) *TokenRepository { return &TokenRepository{db: db} }

func revokeUserTokens(ctx context.Context, db pgtx.Querier, userID uuid.UUID) error {
	_, err := postgres.Exec(ctx, db, constraints, `
		UPDATE personal_access_tokens
		SET revoked_at = now()
		WHERE user_id = $1
			AND revoked_at IS NULL`, userID)
	if err != nil {
		return err
	}
	return revokeUserGrants(ctx, db, userID)
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

func (s *TokenRepository) Insert(ctx context.Context, nt accesstypes.NewToken) (accesstypes.Token, error) {
	var days *int32
	if nt.ExpiresInDays != nil {
		days = new(int32(*nt.ExpiresInDays))
	}
	return postgres.One(ctx, s.db.From(ctx), constraints, scanToken, `
		WITH t AS (
			INSERT INTO personal_access_tokens (id, user_id, name, prefix, token_hash, scopes, expires_at, node_ids, catalog_admin)
			SELECT $1, $2, $3, $4, $5, $6, now() + make_interval(days => $7::int), $8::uuid[], u.is_superadmin
			FROM users u
			WHERE u.id = $2
			RETURNING *
		)
		SELECT t.id, t.user_id, u.email, t.name, t.prefix, t.scopes, rfc3339(t.created_at) AS created_at,
			rfc3339(t.expires_at) AS expires_at, rfc3339(t.last_used_at) AS last_used_at,
			rfc3339(t.revoked_at) AS revoked_at,
			CASE
				WHEN t.revoked_at IS NOT NULL THEN 'revoked'
				WHEN t.expires_at <= now() THEN 'expired'
				ELSE 'active' END AS status, t.node_ids, t.catalog_admin AND u.is_superadmin AS catalog_admin
		FROM t
		JOIN users u ON u.id = t.user_id`, nt.ID, nt.UserID, nt.Name, nt.Prefix, nt.Hash[:], nt.Scopes.Strings(), days, nt.Nodes)
}

func (s *TokenRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]accesstypes.Token, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, scanToken, `
		SELECT t.id, t.user_id, u.email, t.name, t.prefix, t.scopes, rfc3339(t.created_at) AS created_at,
			rfc3339(t.expires_at) AS expires_at, rfc3339(t.last_used_at) AS last_used_at,
			rfc3339(t.revoked_at) AS revoked_at,
			CASE
				WHEN t.revoked_at IS NOT NULL THEN 'revoked'
				WHEN t.expires_at <= now() THEN 'expired'
				ELSE 'active' END AS status, t.node_ids, t.catalog_admin AND u.is_superadmin AS catalog_admin
		FROM personal_access_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.user_id = $1
		ORDER BY t.created_at DESC, t.id DESC`, userID)
}

func (s *TokenRepository) List(ctx context.Context, f accesstypes.TokenFilter) (accesstypes.Page[accesstypes.Token], error) {
	pattern := likePrefix(f.Prefix)
	items, err := postgres.List(ctx, s.db.From(ctx), constraints, scanToken, `
		SELECT t.id, t.user_id, u.email, t.name, t.prefix, t.scopes, rfc3339(t.created_at) AS created_at,
			rfc3339(t.expires_at) AS expires_at, rfc3339(t.last_used_at) AS last_used_at,
			rfc3339(t.revoked_at) AS revoked_at,
			CASE
				WHEN t.revoked_at IS NOT NULL THEN 'revoked'
				WHEN t.expires_at <= now() THEN 'expired'
				ELSE 'active' END AS status, t.node_ids, t.catalog_admin AND u.is_superadmin AS catalog_admin
		FROM personal_access_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.prefix LIKE $1
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT $2
		OFFSET $3`, pattern, int64(f.Page.Limit), int64(f.Page.Offset))
	if err != nil {
		return accesstypes.Page[accesstypes.Token]{}, err
	}
	total, err := postgres.One(ctx, s.db.From(ctx), constraints, pgx.RowTo[int64], `
		SELECT count(*)
		FROM personal_access_tokens t
		WHERE t.prefix LIKE $1`, pattern)
	if err != nil {
		return accesstypes.Page[accesstypes.Token]{}, err
	}
	return accesstypes.Page[accesstypes.Token]{Items: items, Total: uint64(max(total, 0)), Limit: f.Page.Limit, Offset: f.Page.Offset}, nil
}

func (s *TokenRepository) Revoke(ctx context.Context, id uuid.UUID, owner *uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE personal_access_tokens
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE id = $1
			AND ($2::uuid IS NULL OR user_id = $2)`, id, owner)
}

func (s *TokenRepository) AddNode(ctx context.Context, id, nodeID uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE personal_access_tokens
		SET node_ids = array_append(live_node_ids(node_ids), $2)
		WHERE id = $1
			AND node_ids IS NOT NULL
			AND cardinality(live_node_ids(node_ids)) < 20
			AND NOT $2 = ANY (node_ids)`, id, nodeID)
}

func (s *TokenRepository) FindValid(ctx context.Context, tokenHash [32]byte) (*accesstypes.TokenUser, error) {
	var stale bool
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (accesstypes.TokenUser, error) {
		var t accesstypes.TokenUser
		var scopes []string
		var err error
		if t.User, err = scanUserWith(r, []any{&t.TokenID, &scopes, &t.Nodes, &t.CatalogAdmin, &stale}, nil); err != nil {
			return t, err
		}
		t.Scopes, err = parseScopes(scopes)
		return t, err
	}, `
		SELECT t.id, t.scopes, t.node_ids, t.catalog_admin,
			(t.last_used_at IS NULL OR t.last_used_at < now() - interval '60 seconds') AS stale, u.id, u.email,
			u.display_name, u.status, u.is_superadmin, u.is_service, u.password_hash IS NOT NULL AS has_password,
			rfc3339(u.created_at) AS created_at, rfc3339(u.updated_at) AS updated_at
		FROM personal_access_tokens t
		JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = $1
			AND t.revoked_at IS NULL
			AND (t.expires_at IS NULL OR t.expires_at > now())
			AND u.status = 'active'`, tokenHash[:])
	if err != nil || found == nil {
		return nil, err
	}
	if stale {
		if _, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
			UPDATE personal_access_tokens
			SET last_used_at = now()
			WHERE id = $1`, found.TokenID); err != nil {
			return nil, err
		}
	}
	return found, nil
}
