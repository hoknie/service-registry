package services

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"time"

	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
)

var (
	errInvalidGrant  = accesstypes.OAuthFail("invalid_grant", "the code or refresh token is invalid, expired, revoked or was issued to another client")
	errInvalidClient = accesstypes.OAuthFail("invalid_client", "unknown client_id")
)

func (s *AuthorizationServerService) Token(ctx context.Context, req accesstypes.TokenRequest) (accesstypes.TokenPair, error) {
	if err := s.enabled(); err != nil {
		return accesstypes.TokenPair{}, err
	}
	switch req.GrantType {
	case "authorization_code", "refresh_token":
	case "":
		return accesstypes.TokenPair{}, accesstypes.OAuthFail("invalid_request", "grant_type is required")
	default:
		return accesstypes.TokenPair{}, accesstypes.OAuthFail("unsupported_grant_type",
			"grant_type must be authorization_code or refresh_token")
	}
	clientID, err := s.knownClient(ctx, req.ClientID)
	if err != nil {
		return accesstypes.TokenPair{}, err
	}
	if req.Resource != "" && req.Resource != s.resource() {
		return accesstypes.TokenPair{}, accesstypes.OAuthFail("invalid_target", "resource must be "+s.resource())
	}
	if req.GrantType == "refresh_token" {
		return s.refresh(ctx, clientID, req.RefreshToken)
	}
	return s.exchange(ctx, clientID, req)
}

func (s *AuthorizationServerService) knownClient(ctx context.Context, raw string) (uuid.UUID, error) {
	id, err := s.tokenClient(ctx, raw)
	if err != nil {
		return uuid.Nil, err
	}
	if id == nil || (uuid.Validate(raw) != nil && !s.documentHostAllowed(raw)) {
		return uuid.Nil, errInvalidClient
	}
	return *id, nil
}

func (s *AuthorizationServerService) exchange(ctx context.Context, clientID uuid.UUID, req accesstypes.TokenRequest) (accesstypes.TokenPair, error) {
	if req.Code == "" || req.RedirectURI == "" || req.CodeVerifier == "" {
		return accesstypes.TokenPair{}, accesstypes.OAuthFail("invalid_request", "code, redirect_uri and code_verifier are required")
	}
	taken, err := s.authorizationRepository.TakeCode(ctx, auth.TokenHash(req.Code))
	if err != nil {
		return accesstypes.TokenPair{}, apperr.Wrap(err)
	}
	if taken == nil {
		return accesstypes.TokenPair{}, errInvalidGrant
	}
	if taken.Exchanged {
		if taken.GrantID != nil {
			if err := s.grantRepository.Revoke(ctx, *taken.GrantID, nil); err != nil && !errors.Is(err, apperr.ErrNotFound) {
				return accesstypes.TokenPair{}, apperr.Wrap(err)
			}
		}
		return accesstypes.TokenPair{}, errInvalidGrant
	}
	if taken.ClientID != clientID || taken.RedirectURI != req.RedirectURI || !accesstypes.VerifierMatches(req.CodeVerifier, taken.CodeChallenge) {
		return accesstypes.TokenPair{}, errInvalidGrant
	}
	var pair accesstypes.TokenPair
	err = s.db.InTx(ctx, func(ctx context.Context) error {
		refresh, err := auth.GenerateAppRefreshToken()
		if err != nil {
			return apperr.Internalf("no randomness: %v", err)
		}
		grant := accesstypes.NewGrant{ID: uuid.Must(uuid.NewV7()), UserID: taken.UserID, ClientID: clientID,
			Scopes: taken.Scopes, Nodes: taken.Nodes, RefreshHash: refresh.Hash, RefreshDays: s.cfg.MCPOAuth.RefreshDays}
		if err := s.grantRepository.Insert(ctx, grant); err != nil {
			return err
		}
		if err := s.authorizationRepository.SetGrant(ctx, taken.AuthorizationID, grant.ID); err != nil {
			return err
		}
		pair, err = s.issueAccess(ctx, grant.ID, grant.Scopes, refresh.Secret)
		return err
	})
	return pair, apperr.Wrap(err)
}

func (s *AuthorizationServerService) issueAccess(ctx context.Context, grantID uuid.UUID, scopes accesstypes.Scopes, refresh string) (accesstypes.TokenPair, error) {
	access, err := auth.GenerateAppAccessToken()
	if err != nil {
		return accesstypes.TokenPair{}, apperr.Internalf("no randomness: %v", err)
	}
	ttl := s.cfg.MCPOAuth.AccessTTLSecs
	if err := s.grantRepository.InsertAccess(ctx, grantID, access.Hash, ttl); err != nil {
		return accesstypes.TokenPair{}, err
	}
	return accesstypes.TokenPair{Access: access.Secret, Refresh: refresh, ExpiresIn: ttl, Scopes: scopes}, nil
}

func (s *AuthorizationServerService) refresh(ctx context.Context, clientID uuid.UUID, presented string) (accesstypes.TokenPair, error) {
	if presented == "" {
		return accesstypes.TokenPair{}, accesstypes.OAuthFail("invalid_request", "refresh_token is required")
	}
	old, ok := auth.ParseAppRefreshToken(presented)
	if !ok {
		return accesstypes.TokenPair{}, errInvalidGrant
	}
	var pair accesstypes.TokenPair
	replayed := false
	err := s.db.InTx(ctx, func(ctx context.Context) error {
		fresh, err := auth.GenerateAppRefreshToken()
		if err != nil {
			return apperr.Internalf("no randomness: %v", err)
		}
		grant, err := s.grantRepository.Rotate(ctx, clientID, old, fresh.Hash, s.cfg.MCPOAuth.RefreshDays)
		if err != nil {
			return err
		}
		if grant == nil {
			replayed = true
			return nil
		}
		pair, err = s.issueAccess(ctx, grant.ID, grant.Scopes, fresh.Secret)
		return err
	})
	if err != nil {
		return accesstypes.TokenPair{}, apperr.Wrap(err)
	}
	if replayed {
		if _, err := s.grantRepository.RevokeReplayed(ctx, old); err != nil {
			return accesstypes.TokenPair{}, apperr.Wrap(err)
		}
		return accesstypes.TokenPair{}, errInvalidGrant
	}
	return pair, nil
}

func (s *AuthorizationServerService) Revoke(ctx context.Context, rawClientID, token string) error {
	if err := s.enabled(); err != nil {
		return err
	}
	id, err := s.tokenClient(ctx, rawClientID)
	if err != nil || id == nil {
		return err
	}
	clientID := *id
	if hash, ok := auth.ParseAppRefreshToken(token); ok {
		return apperr.Wrap(s.grantRepository.RevokeByRefresh(ctx, clientID, hash))
	}
	if hash, ok := auth.ParseAppAccessToken(token); ok {
		return apperr.Wrap(s.grantRepository.DeleteAccess(ctx, clientID, hash))
	}
	return nil
}

func (s *AuthorizationServerService) authorizingClient(ctx context.Context, q accesstypes.AuthorizeQuery) (*accesstypes.OAuthClientRecord, *accesstypes.ClientMetadataError, error) {
	invalid := &accesstypes.ClientMetadataError{Code: accesstypes.CodeInvalidClient}
	if id, err := uuid.Parse(q.ClientID); err == nil {
		client, err := s.clientRepository.Find(ctx, id)
		if err != nil || client != nil {
			return client, nil, err
		}
		return nil, invalid, nil
	}
	if !s.cfg.MCPOAuth.ClientMetadata || !accesstypes.ValidClientMetadataURL(q.ClientID) {
		return nil, invalid, nil
	}
	return s.documentClient(ctx, q.PeerIP, q.ClientID)
}

func (s *AuthorizationServerService) documentClient(ctx context.Context, ip, clientID string) (*accesstypes.OAuthClientRecord, *accesstypes.ClientMetadataError, error) {
	u, err := url.Parse(clientID)
	if err != nil {
		return nil, &accesstypes.ClientMetadataError{Code: accesstypes.CodeInvalidClient}, nil
	}
	if !s.cfg.MCPOAuth.ClientMetadataHostAllowed(u.Hostname()) {
		return nil, &accesstypes.ClientMetadataError{Code: accesstypes.CodeClientNotAllowed}, nil
	}
	cached, err := s.clientRepository.FindDocument(ctx, clientID)
	if err != nil {
		return nil, nil, err
	}
	if cached != nil && cached.Fresh {
		return cached, nil, nil
	}
	meta, refusal := s.fetchDocument(ctx, ip, clientID)
	if refusal != nil {
		slog.Warn("client metadata document refused", "client_id", clientID, "code", refusal.Code,
			"reason", refusal.Reason, "detail", refusal.Detail)
		return nil, refusal, nil
	}
	life := accesstypes.MetadataLifetime(meta.cacheControl)
	client, err := s.clientRepository.SaveDocument(ctx, uuid.Must(uuid.NewV7()), clientID, meta.ClientMetadata,
		int64(life/time.Second))
	if err != nil {
		return nil, nil, err
	}
	return &client, nil, nil
}

type fetchedClient struct {
	accesstypes.ClientMetadata
	cacheControl string
}

func (s *AuthorizationServerService) fetchDocument(ctx context.Context, ip, clientID string) (fetchedClient, *accesstypes.ClientMetadataError) {
	if s.metadata == nil {
		return fetchedClient{}, accesstypes.MetadataUnavailable(accesstypes.ReasonFetchFailed, "no metadata fetcher")
	}
	if !s.fetches.Hit(ip, time.Now()) {
		return fetchedClient{}, accesstypes.MetadataUnavailable(accesstypes.ReasonRateLimited, "too many document fetches from "+ip)
	}
	doc, err := s.metadata.Fetch(ctx, clientID)
	if err != nil {
		reason := accesstypes.ReasonFetchFailed
		var failure *accesstypes.ClientMetadataError
		if errors.As(err, &failure) {
			reason = failure.Reason
		}
		return fetchedClient{}, accesstypes.MetadataUnavailable(reason, err.Error())
	}
	meta, refusal := accesstypes.ParseClientMetadataDocument(clientID, doc.Body)
	if refusal != nil {
		return fetchedClient{}, refusal
	}
	return fetchedClient{ClientMetadata: meta, cacheControl: doc.CacheControl}, nil
}

func (s *AuthorizationServerService) tokenClient(ctx context.Context, raw string) (*uuid.UUID, error) {
	if id, err := uuid.Parse(raw); err == nil {
		ok, err := s.clientRepository.Exists(ctx, id)
		if err != nil || !ok {
			return nil, apperr.Wrap(err)
		}
		return &id, nil
	}
	if !accesstypes.ValidClientMetadataURL(raw) {
		return nil, nil
	}
	id, err := s.clientRepository.DocumentClient(ctx, raw)
	return id, apperr.Wrap(err)
}

func (s *AuthorizationServerService) metadataHint(v url.Values, clientID string, refusal *accesstypes.ClientMetadataError) {
	u, err := url.Parse(clientID)
	if err != nil || !accesstypes.ValidClientMetadataURL(clientID) {
		return
	}
	switch {
	case refusal.Code == accesstypes.CodeClientNotAllowed:
		v.Set("hint", accesstypes.HintMetadataHosts)
	case refusal.Code == accesstypes.CodeInvalidClient && !s.cfg.MCPOAuth.ClientMetadata:
		v.Set("hint", accesstypes.HintMetadataDisabled)
	case refusal.Code == accesstypes.CodeClientMetadataUnavailable && refusal.Reason != "":
		v.Set("hint", accesstypes.HintMetadataUnavailable)
		v.Set("reason", refusal.Reason)
	case refusal.Code == accesstypes.CodeClientMetadataInvalid && refusal.Reason != "":
		v.Set("hint", accesstypes.HintMetadataInvalid)
		v.Set("reason", refusal.Reason)
	default:
		return
	}
	v.Set("host", u.Hostname())
}

func (s *AuthorizationServerService) documentHostAllowed(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && s.cfg.MCPOAuth.ClientMetadata && s.cfg.MCPOAuth.ClientMetadataHostAllowed(u.Hostname())
}
