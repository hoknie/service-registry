package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type OAuthGrantRepository struct{ db *pgtx.DB }

func NewOAuthGrantRepository(db *pgtx.DB) *OAuthGrantRepository { return &OAuthGrantRepository{db: db} }

func revokeUserGrants(ctx context.Context, db pgtx.Querier, userID uuid.UUID) error {
	_, err := postgres.Exec(ctx, db, constraints, `
		UPDATE oauth_grants
		SET revoked_at = now()
		WHERE user_id = $1
			AND revoked_at IS NULL`, userID)
	return err
}

func (s *OAuthGrantRepository) Insert(ctx context.Context, g accesstypes.NewGrant) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO oauth_grants (id, user_id, client_id, scopes, node_ids, refresh_hash, refresh_expires_at, catalog_admin)
		SELECT $1, $2, $3, $4, $5::uuid[], $6, now() + make_interval(days => $7::int), u.is_superadmin
		FROM users u
		WHERE u.id = $2`,
		g.ID, g.UserID, g.ClientID, g.Scopes.Strings(), g.Nodes, g.RefreshHash[:], int32(g.RefreshDays))
	return err
}

func (s *OAuthGrantRepository) InsertAccess(ctx context.Context, grantID uuid.UUID, hash [32]byte, ttlSecs uint32) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO oauth_access_tokens (id, grant_id, token_hash, expires_at)
		VALUES ($1, $2, $3, now() + make_interval(secs => $4))`,
		uuid.Must(uuid.NewV7()), grantID, hash[:], float64(ttlSecs))
	return err
}

func (s *OAuthGrantRepository) Rotate(ctx context.Context, clientID uuid.UUID, old, fresh [32]byte, days uint32) (*accesstypes.NewGrant, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (accesstypes.NewGrant, error) {
		var g accesstypes.NewGrant
		var scopes []string
		if err := r.Scan(&g.ID, &g.UserID, &g.ClientID, &scopes, &g.Nodes); err != nil {
			return g, err
		}
		var err error
		g.Scopes, err = parseScopes(scopes)
		return g, err
	}, `
		UPDATE oauth_grants g
		SET previous_refresh_hash = g.refresh_hash, refresh_hash = $3,
			refresh_expires_at = now() + make_interval(days => $4::int)
		FROM users u
		WHERE g.refresh_hash = $2
			AND g.client_id = $1
			AND g.revoked_at IS NULL
			AND g.refresh_expires_at > now()
			AND u.id = g.user_id
			AND u.status = 'active'
		RETURNING g.id, g.user_id, g.client_id, g.scopes, g.node_ids`, clientID, old[:], fresh[:], int32(days))
}

func (s *OAuthGrantRepository) RevokeReplayed(ctx context.Context, refreshHash [32]byte) (bool, error) {
	n, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE oauth_grants
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE previous_refresh_hash = $1`, refreshHash[:])
	return n > 0, err
}

func (s *OAuthGrantRepository) RevokeByRefresh(ctx context.Context, clientID uuid.UUID, refreshHash [32]byte) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		UPDATE oauth_grants
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE client_id = $1
			AND (refresh_hash = $2 OR previous_refresh_hash = $2)`, clientID, refreshHash[:])
	return err
}

func (s *OAuthGrantRepository) DeleteAccess(ctx context.Context, clientID uuid.UUID, hash [32]byte) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM oauth_access_tokens t
		USING oauth_grants g
		WHERE t.token_hash = $2
			AND g.id = t.grant_id
			AND g.client_id = $1`, clientID, hash[:])
	return err
}

func (s *OAuthGrantRepository) Revoke(ctx context.Context, id uuid.UUID, owner *uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE oauth_grants
		SET revoked_at = COALESCE(revoked_at, now())
		WHERE id = $1
			AND ($2::uuid IS NULL OR user_id = $2)`, id, owner)
}

func (s *OAuthGrantRepository) RevokeForUser(ctx context.Context, userID uuid.UUID) error {
	return revokeUserGrants(ctx, s.db.From(ctx), userID)
}

func (s *OAuthGrantRepository) FindValidAccess(ctx context.Context, hash [32]byte) (*accesstypes.AppPrincipal, error) {
	var stale bool
	found, err := postgres.Find(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (accesstypes.AppPrincipal, error) {
		var p accesstypes.AppPrincipal
		var err error
		p.User, err = scanUserWith(r, []any{&p.GrantID, &p.Nodes, &p.CatalogAdmin, &stale}, nil)
		return p, err
	}, `
		SELECT g.id, g.node_ids, g.catalog_admin, (g.last_used_at IS NULL OR g.last_used_at < now() - interval '60 seconds') AS stale,
			u.id, u.email, u.display_name, u.status, u.is_superadmin, u.is_service, u.password_hash IS NOT NULL AS has_password,
			rfc3339(u.created_at) AS created_at, rfc3339(u.updated_at) AS updated_at
		FROM oauth_access_tokens t
		JOIN oauth_grants g ON g.id = t.grant_id
		JOIN users u ON u.id = g.user_id
		WHERE t.token_hash = $1
			AND t.expires_at > now()
			AND g.revoked_at IS NULL
			AND u.status = 'active'`, hash[:])
	if err != nil || found == nil {
		return nil, err
	}
	if stale {
		if _, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
			UPDATE oauth_grants
			SET last_used_at = now()
			WHERE id = $1`, found.GrantID); err != nil {
			return nil, err
		}
	}
	return found, nil
}

func (s *OAuthGrantRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]accesstypes.AppGrant, error) {
	return postgres.List(ctx, s.db.From(ctx), constraints, func(r pgx.CollectableRow) (accesstypes.AppGrant, error) {
		var g accesstypes.AppGrant
		var scopes, redirects []string
		var st string
		err := r.Scan(&g.ID, &g.UserID, &g.ClientID, &g.ClientName, &g.ClientMetadataURL, &redirects, &scopes, &g.NodeIDs, &g.CreatedAt,
			&g.LastUsedAt, &g.ExpiresAt, &g.RevokedAt, &st)
		if err != nil {
			return g, err
		}
		if len(redirects) > 0 {
			g.RedirectHost = accesstypes.RedirectHost(redirects[0])
		}
		var ok bool
		if g.Status, ok = accesstypes.ParseTokenStatus(st); !ok {
			return g, apperr.InternalDetail("unknown grant status " + st)
		}
		g.Scopes, err = parseScopes(scopes)
		return g, err
	}, `
		SELECT g.id, g.user_id, g.client_id, c.name, c.metadata_url, c.redirect_uris, g.scopes, g.node_ids, rfc3339(g.created_at) AS created_at,
			rfc3339(g.last_used_at) AS last_used_at, rfc3339(g.refresh_expires_at) AS expires_at,
			rfc3339(g.revoked_at) AS revoked_at,
			CASE
				WHEN g.revoked_at IS NOT NULL THEN 'revoked'
				WHEN g.refresh_expires_at <= now() THEN 'expired'
				ELSE 'active' END AS status
		FROM oauth_grants g
		JOIN oauth_clients c ON c.id = g.client_id
		WHERE g.user_id = $1
		ORDER BY g.created_at DESC, g.id DESC`, userID)
}

func (s *OAuthGrantRepository) Prune(ctx context.Context) (int64, error) {
	tokens, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM oauth_access_tokens
		WHERE expires_at <= now()`)
	if err != nil {
		return 0, err
	}
	grants, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM oauth_grants
		WHERE revoked_at < now() - interval '30 days'
			OR refresh_expires_at < now() - interval '30 days'`)
	return tokens + grants, err
}
