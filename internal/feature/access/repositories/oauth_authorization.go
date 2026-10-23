package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/postgres"
	"svc-registry/pkg/pgtx"
)

type OAuthAuthorizationRepository struct{ db *pgtx.DB }

func NewOAuthAuthorizationRepository(db *pgtx.DB) *OAuthAuthorizationRepository {
	return &OAuthAuthorizationRepository{db: db}
}

func scanAuthorizationRequest(row pgx.CollectableRow) (accesstypes.AuthorizationRequest, error) {
	var a accesstypes.AuthorizationRequest
	var scopes []string
	if err := row.Scan(&a.ID, &a.ClientID, &a.ClientName, &a.ClientMetadataURL, &a.RedirectURI, &a.State, &a.Resource, &scopes, &a.ExpiresAt); err != nil {
		return a, err
	}
	var err error
	a.Scopes, err = parseScopes(scopes)
	return a, err
}

func (s *OAuthAuthorizationRepository) Insert(ctx context.Context, n accesstypes.NewAuthorization) error {
	_, err := postgres.Exec(ctx, s.db.From(ctx), constraints, `
		INSERT INTO oauth_authorizations (id, client_id, redirect_uri, state, code_challenge, resource, scopes, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now() + make_interval(secs => $8))`,
		n.ID, n.ClientID, n.RedirectURI, n.State, n.CodeChallenge, n.Resource, n.Scopes.Strings(),
		float64(accesstypes.OAuthRequestTTLSecs))
	return err
}

func (s *OAuthAuthorizationRepository) FindPending(ctx context.Context, id uuid.UUID) (*accesstypes.AuthorizationRequest, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanAuthorizationRequest, `
		SELECT a.id, a.client_id, c.name, c.metadata_url, a.redirect_uri, a.state, a.resource, a.scopes, rfc3339(a.expires_at) AS expires_at
		FROM oauth_authorizations a
		JOIN oauth_clients c ON c.id = a.client_id
		WHERE a.id = $1
			AND a.decided_at IS NULL
			AND a.expires_at > now()`, id)
}

func (s *OAuthAuthorizationRepository) Approve(ctx context.Context, id, userID uuid.UUID, nodes []uuid.UUID, codeHash [32]byte) (*accesstypes.AuthorizationRequest, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanAuthorizationRequest, `
		UPDATE oauth_authorizations a
		SET decided_at = now(), user_id = $2, node_ids = $3::uuid[], code_hash = $4,
			expires_at = now() + make_interval(secs => $5)
		FROM oauth_clients c
		WHERE a.id = $1
			AND c.id = a.client_id
			AND a.decided_at IS NULL
			AND a.expires_at > now()
		RETURNING a.id, a.client_id, c.name, c.metadata_url, a.redirect_uri, a.state, a.resource, a.scopes,
			rfc3339(a.expires_at) AS expires_at`, id, userID, nodes, codeHash[:], float64(accesstypes.OAuthCodeTTLSecs))
}

func (s *OAuthAuthorizationRepository) Deny(ctx context.Context, id, userID uuid.UUID) (*accesstypes.AuthorizationRequest, error) {
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanAuthorizationRequest, `
		UPDATE oauth_authorizations a
		SET decided_at = now(), user_id = $2
		FROM oauth_clients c
		WHERE a.id = $1
			AND c.id = a.client_id
			AND a.decided_at IS NULL
			AND a.expires_at > now()
		RETURNING a.id, a.client_id, c.name, c.metadata_url, a.redirect_uri, a.state, a.resource, a.scopes,
			rfc3339(a.expires_at) AS expires_at`, id, userID)
}

func scanApprovedCode(row pgx.CollectableRow) (accesstypes.ApprovedCode, error) {
	var a accesstypes.ApprovedCode
	var scopes []string
	err := row.Scan(&a.AuthorizationID, &a.ClientID, &a.UserID, &a.RedirectURI, &a.CodeChallenge, &a.Resource, &scopes,
		&a.Nodes, &a.GrantID, &a.Exchanged)
	if err != nil {
		return a, err
	}
	a.Scopes, err = parseScopes(scopes)
	return a, err
}

func (s *OAuthAuthorizationRepository) TakeCode(ctx context.Context, codeHash [32]byte) (*accesstypes.ApprovedCode, error) {
	taken, err := postgres.Find(ctx, s.db.From(ctx), constraints, scanApprovedCode, `
		UPDATE oauth_authorizations
		SET exchanged_at = now()
		WHERE code_hash = $1
			AND exchanged_at IS NULL
			AND expires_at > now()
		RETURNING id, client_id, user_id, redirect_uri, code_challenge, resource, scopes, node_ids, grant_id, false`, codeHash[:])
	if err != nil || taken != nil {
		return taken, err
	}
	return postgres.Find(ctx, s.db.From(ctx), constraints, scanApprovedCode, `
		SELECT id, client_id, user_id, redirect_uri, code_challenge, resource, scopes, node_ids, grant_id, true
		FROM oauth_authorizations
		WHERE code_hash = $1
			AND exchanged_at IS NOT NULL`, codeHash[:])
}

func (s *OAuthAuthorizationRepository) SetGrant(ctx context.Context, id, grantID uuid.UUID) error {
	return postgres.ExecFound(ctx, s.db.From(ctx), constraints, `
		UPDATE oauth_authorizations
		SET grant_id = $2
		WHERE id = $1`, id, grantID)
}

func (s *OAuthAuthorizationRepository) Prune(ctx context.Context) (int64, error) {
	return postgres.Exec(ctx, s.db.From(ctx), constraints, `
		DELETE FROM oauth_authorizations
		WHERE expires_at < now() - interval '1 hour'`)
}
