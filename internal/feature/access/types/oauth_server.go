package types

import (
	"crypto/subtle"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

const (
	OAuthClientMaxRedirects = 10
	OAuthRedirectMaxLen     = 2000
	OAuthClientNameMaxLen   = 100
	OAuthStateMaxLen        = 500
	OAuthRequestTTLSecs     = 600
	OAuthCodeTTLSecs        = 300
	OAuthUIPath             = "/oauth/authorize"
	OAuthResourcePath       = "/api/mcp"
)

type OAuthError struct {
	Code        string
	Description string
	RetryAfter  uint64
}

func (e *OAuthError) Error() string { return e.Code + ": " + e.Description }

func OAuthFail(code, description string) *OAuthError {
	return &OAuthError{Code: code, Description: description}
}

type ClientMetadata struct {
	Name                    *string
	RedirectURIs            []string
	TokenEndpointAuthMethod *string
	GrantTypes              []string
	ResponseTypes           []string
}

type OAuthClientRecord struct {
	ID           uuid.UUID
	Name         *string
	RedirectURIs []string
	MetadataURL  *string
	Fresh        bool
	CreatedAt    string
	CreatedUnix  int64
}

type AuthorizeQuery struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	CodeChallenge       string
	CodeChallengeMethod string
	Scope               string
	State               string
	Resource            string
	PeerIP              string
	Superadmin          bool
}

type NewAuthorization struct {
	ID            uuid.UUID
	ClientID      uuid.UUID
	RedirectURI   string
	State         *string
	CodeChallenge string
	Resource      *string
	Scopes        Scopes
	TTLSecs       int
}

type AuthorizationRequest struct {
	ID                uuid.UUID
	ClientID          uuid.UUID
	ClientName        *string
	ClientMetadataURL *string
	RedirectURI       string
	State             *string
	Resource          *string
	Scopes            Scopes
	ExpiresAt         string
	MCPServers        []ServerRef
}

type ApprovedCode struct {
	AuthorizationID uuid.UUID
	ClientID        uuid.UUID
	UserID          uuid.UUID
	RedirectURI     string
	CodeChallenge   string
	Resource        *string
	Scopes          Scopes
	Nodes           []uuid.UUID
	MCPServers      ServerSet
	GrantID         *uuid.UUID
	Exchanged       bool
}

type TokenRequest struct {
	GrantType    string
	Code         string
	RedirectURI  string
	ClientID     string
	CodeVerifier string
	RefreshToken string
	Resource     string
}

type TokenPair struct {
	Access    string
	Refresh   string
	ExpiresIn uint32
	Scopes    Scopes
}

type NewGrant struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	ClientID    uuid.UUID
	Scopes      Scopes
	Nodes       []uuid.UUID
	MCPServers  ServerSet
	RefreshHash [32]byte
	RefreshDays uint32
}

type AppGrant struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	ClientID          uuid.UUID
	ClientName        *string
	ClientMetadataURL *string
	RedirectHost      string
	Scopes            Scopes
	NodeIDs           []uuid.UUID
	Nodes             []NodeRef
	CreatedAt         string
	LastUsedAt        *string
	ExpiresAt         string
	RevokedAt         *string
	Status            TokenStatus
	MCPServerSet      ServerSet
	MCPServers        []ServerRef
}

type AppPrincipal struct {
	GrantID      uuid.UUID
	Nodes        []uuid.UUID
	CatalogAdmin bool
	MCPServers   ServerSet
	User         User
}

var (
	codeChallengeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	verifierRe      = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
	dottedSchemeRe  = regexp.MustCompile(`^[a-z][a-z0-9+.-]*\.[a-z0-9+.-]+$`)
	letterSchemeRe  = regexp.MustCompile(`^[a-z]+$`)
)

var forbiddenSchemes = []string{"http", "https", "javascript", "data", "file", "vbscript"}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func ValidRedirectURI(raw string) bool {
	if raw == "" || len(raw) > OAuthRedirectMaxLen || strings.ContainsAny(raw, "# \t\r\n") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return false
	}
	switch scheme := strings.ToLower(u.Scheme); {
	case scheme == "https":
		return u.Host != ""
	case scheme == "http":
		return isLoopback(u.Hostname())
	case slices.Contains(forbiddenSchemes, scheme):
		return false
	case dottedSchemeRe.MatchString(scheme), letterSchemeRe.MatchString(scheme):
		return u.Opaque != "" || u.Path != "" || u.Host != ""
	}
	return false
}

func RedirectMatches(registered []string, presented string) bool {
	if slices.Contains(registered, presented) {
		return true
	}
	p, err := url.Parse(presented)
	if err != nil || p.Scheme != "http" || !isLoopback(p.Hostname()) {
		return false
	}
	for _, r := range registered {
		u, err := url.Parse(r)
		if err == nil && u.Scheme == "http" && u.Hostname() == p.Hostname() && u.EscapedPath() == p.EscapedPath() &&
			u.RawQuery == p.RawQuery {
			return true
		}
	}
	return false
}

func ValidCodeChallenge(challenge string) bool { return codeChallengeRe.MatchString(challenge) }

func VerifierMatches(verifier, challenge string) bool {
	if !verifierRe.MatchString(verifier) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(oauth2.S256ChallengeFromVerifier(verifier)), []byte(challenge)) == 1
}

func ValidateClientMetadata(in ClientMetadata) (ClientMetadata, *OAuthError) {
	if len(in.RedirectURIs) == 0 || len(in.RedirectURIs) > OAuthClientMaxRedirects {
		return ClientMetadata{}, OAuthFail("invalid_redirect_uri", "redirect_uris must hold 1 to 10 addresses")
	}
	for _, r := range in.RedirectURIs {
		if !ValidRedirectURI(r) {
			return ClientMetadata{}, OAuthFail("invalid_redirect_uri",
				"each redirect URI must be https, http on a loopback host or a private-use scheme, without a fragment, at most 2000 characters")
		}
	}
	if in.TokenEndpointAuthMethod != nil && *in.TokenEndpointAuthMethod != "none" {
		return ClientMetadata{}, OAuthFail("invalid_client_metadata", `only public clients are supported: token_endpoint_auth_method must be "none"`)
	}
	for _, g := range in.GrantTypes {
		if g != "authorization_code" && g != "refresh_token" {
			return ClientMetadata{}, OAuthFail("invalid_client_metadata", "grant_types may hold only authorization_code and refresh_token")
		}
	}
	for _, r := range in.ResponseTypes {
		if r != "code" {
			return ClientMetadata{}, OAuthFail("invalid_client_metadata", `response_types may hold only "code"`)
		}
	}
	out := ClientMetadata{RedirectURIs: slices.Compact(slices.Clone(in.RedirectURIs))}
	if in.Name != nil {
		if name := strings.TrimSpace(*in.Name); name != "" {
			if r := []rune(name); len(r) > OAuthClientNameMaxLen {
				name = strings.TrimSpace(string(r[:OAuthClientNameMaxLen]))
			}
			out.Name = &name
		}
	}
	return out, nil
}

func RedirectHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Host != "" {
		return u.Host
	}
	return u.Scheme + ":"
}
