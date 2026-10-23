package services

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	accessports "svc-registry/internal/feature/access/ports"
	"svc-registry/internal/feature/access/repositories"
	accesstypes "svc-registry/internal/feature/access/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/platform/auth"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
)

type ServerMetadata struct {
	Issuer         string
	Resource       string
	ClientMetadata bool
}

type AuthorizationServerService struct {
	cfg                     *config.Config
	db                      *pgtx.DB
	roots                   accessports.NodeRootsPort
	metadata                accessports.ClientMetadataPort
	registrations           *auth.RegistrationLimiter
	fetches                 *auth.MetadataFetchLimiter
	clientRepository        *repositories.OAuthClientRepository
	authorizationRepository *repositories.OAuthAuthorizationRepository
	grantRepository         *repositories.OAuthGrantRepository
}

func (s *AuthorizationServerService) enabled() error {
	if s.cfg == nil || !s.cfg.MCPOAuth.Enabled {
		return apperr.New(apperr.NotFound)
	}
	return nil
}

func (s *AuthorizationServerService) issuer() string { return s.cfg.Web.PublicURL }

func (s *AuthorizationServerService) resource() string {
	return s.cfg.Web.PublicURL + accesstypes.OAuthResourcePath
}

func (s *AuthorizationServerService) Metadata() (ServerMetadata, error) {
	if err := s.enabled(); err != nil {
		return ServerMetadata{}, err
	}
	return ServerMetadata{Issuer: s.issuer(), Resource: s.resource(), ClientMetadata: s.cfg.MCPOAuth.ClientDocuments()}, nil
}

func (s *AuthorizationServerService) Register(ctx context.Context, ip string, in accesstypes.ClientMetadata) (accesstypes.OAuthClientRecord, error) {
	if err := s.enabled(); err != nil {
		return accesstypes.OAuthClientRecord{}, err
	}
	meta, refusal := accesstypes.ValidateClientMetadata(in)
	if refusal != nil {
		return accesstypes.OAuthClientRecord{}, refusal
	}
	if secs, ok := s.registrations.Hit(ip, time.Now()); !ok {
		return accesstypes.OAuthClientRecord{}, &accesstypes.OAuthError{Code: "rate_limited",
			Description: "too many client registrations from this address", RetryAfter: secs}
	}
	client, err := s.clientRepository.Insert(ctx, uuid.Must(uuid.NewV7()), meta)
	return client, apperr.Wrap(err)
}

func (s *AuthorizationServerService) uiLocation(query url.Values) string {
	return s.issuer() + accesstypes.OAuthUIPath + "?" + query.Encode()
}

func (s *AuthorizationServerService) clientLocation(redirect string, params url.Values, state *string) string {
	if state != nil {
		params.Set("state", *state)
	}
	params.Set("iss", s.issuer())
	u, err := url.Parse(redirect)
	if err != nil {
		return redirect
	}
	q := u.Query()
	for k, v := range params {
		q[k] = v
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *AuthorizationServerService) StartAuthorization(ctx context.Context, q accesstypes.AuthorizeQuery) (string, error) {
	if err := s.enabled(); err != nil {
		return "", err
	}
	client, code, err := s.authorizingClient(ctx, q)
	if err != nil {
		return "", apperr.Wrap(err)
	}
	if code != "" {
		return s.uiLocation(url.Values{"error": {code}}), nil
	}
	if !accesstypes.RedirectMatches(client.RedirectURIs, q.RedirectURI) {
		return s.uiLocation(url.Values{"error": {accesstypes.CodeInvalidRedirectURI}}), nil
	}
	state := optional(q.State)
	refuse := func(code, description string) (string, error) {
		return s.clientLocation(q.RedirectURI, url.Values{"error": {code}, "error_description": {description}}, state), nil
	}
	switch {
	case q.ResponseType != "code":
		return refuse("unsupported_response_type", `response_type must be "code"`)
	case !accesstypes.ValidCodeChallenge(q.CodeChallenge) || q.CodeChallengeMethod != "S256":
		return refuse("invalid_request", "PKCE with code_challenge_method=S256 is required")
	case q.Resource != "" && q.Resource != s.resource():
		return refuse("invalid_request", "resource must be "+s.resource())
	case len(q.State) > accesstypes.OAuthStateMaxLen:
		return refuse("invalid_request", "state must be at most 500 characters")
	}
	for _, sc := range strings.Fields(q.Scope) {
		if sc != string(accesstypes.ScopeMCP) {
			return refuse("invalid_scope", `the only scope is "mcp"`)
		}
	}
	id := uuid.Must(uuid.NewV7())
	err = s.authorizationRepository.Insert(ctx, accesstypes.NewAuthorization{ID: id, ClientID: client.ID,
		RedirectURI: q.RedirectURI, State: state, CodeChallenge: q.CodeChallenge, Resource: optional(q.Resource),
		Scopes: accesstypes.Scopes{accesstypes.ScopeMCP}})
	if err != nil {
		return "", apperr.Wrap(err)
	}
	return s.uiLocation(url.Values{"request": {id.String()}}), nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s *AuthorizationServerService) Request(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (accesstypes.AuthorizationRequest, error) {
	if err := s.enabled(); err != nil {
		return accesstypes.AuthorizationRequest{}, err
	}
	if err := requireSession(p); err != nil {
		return accesstypes.AuthorizationRequest{}, err
	}
	req, err := s.authorizationRepository.FindPending(ctx, id)
	if err != nil {
		return accesstypes.AuthorizationRequest{}, apperr.Wrap(err)
	}
	if req == nil {
		return accesstypes.AuthorizationRequest{}, apperr.New(apperr.NotFound)
	}
	return *req, nil
}

func (s *AuthorizationServerService) Approve(ctx context.Context, p accesstypes.Principal, id uuid.UUID, nodes *[]string) (string, error) {
	if err := s.enabled(); err != nil {
		return "", err
	}
	if err := requireSession(p); err != nil {
		return "", err
	}
	roots, err := accesstypes.ValidateGrantNodes(nodes)
	if err != nil {
		return "", apperr.Wrap(err)
	}
	if roots != nil {
		readable, err := s.roots.CheckRoots(ctx, p, roots)
		if err != nil {
			return "", apperr.Wrap(err)
		}
		if !readable {
			return "", apperr.Wrap(accesstypes.InvalidTokenNodes)
		}
	}
	code, err := auth.RandomString()
	if err != nil {
		return "", apperr.Internalf("no randomness: %v", err)
	}
	req, err := s.authorizationRepository.Approve(ctx, id, p.UserID, roots, auth.TokenHash(code))
	if err != nil {
		return "", apperr.Wrap(err)
	}
	if req == nil {
		return "", apperr.New(apperr.NotFound)
	}
	return s.clientLocation(req.RedirectURI, url.Values{"code": {code}}, req.State), nil
}

func (s *AuthorizationServerService) Deny(ctx context.Context, p accesstypes.Principal, id uuid.UUID) (string, error) {
	if err := s.enabled(); err != nil {
		return "", err
	}
	if err := requireSession(p); err != nil {
		return "", err
	}
	req, err := s.authorizationRepository.Deny(ctx, id, p.UserID)
	if err != nil {
		return "", apperr.Wrap(err)
	}
	if req == nil {
		return "", apperr.New(apperr.NotFound)
	}
	return s.clientLocation(req.RedirectURI, url.Values{"error": {"access_denied"}}, req.State), nil
}

func (s *AuthorizationServerService) Prune(ctx context.Context) (int64, error) {
	authorizations, err := s.authorizationRepository.Prune(ctx)
	if err != nil {
		return 0, apperr.Wrap(err)
	}
	grants, err := s.grantRepository.Prune(ctx)
	if err != nil {
		return 0, apperr.Wrap(err)
	}
	clients, err := s.clientRepository.PruneAbandoned(ctx)
	return authorizations + grants + clients, apperr.Wrap(err)
}
