package responses

import (
	"github.com/google/uuid"

	accessservices "svc-registry/internal/feature/access/services"
	accesstypes "svc-registry/internal/feature/access/types"
)

type ProtectedResource struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	ScopesSupported        []string `json:"scopes_supported"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	ResourceName           string   `json:"resource_name"`
}

func ProtectedResourceOf(m accessservices.ServerMetadata) ProtectedResource {
	return ProtectedResource{Resource: m.Resource, AuthorizationServers: []string{m.Issuer}, ScopesSupported: []string{"mcp"},
		BearerMethodsSupported: []string{"header"}, ResourceName: "svc-registry"}
}

type AuthorizationServer struct {
	Issuer                                 string   `json:"issuer"`
	AuthorizationEndpoint                  string   `json:"authorization_endpoint"`
	TokenEndpoint                          string   `json:"token_endpoint"`
	RegistrationEndpoint                   string   `json:"registration_endpoint"`
	RevocationEndpoint                     string   `json:"revocation_endpoint"`
	ResponseTypesSupported                 []string `json:"response_types_supported"`
	GrantTypesSupported                    []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported          []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported      []string `json:"token_endpoint_auth_methods_supported"`
	RevocationEndpointAuthMethodsSupported []string `json:"revocation_endpoint_auth_methods_supported"`
	ScopesSupported                        []string `json:"scopes_supported"`
	IssParameterSupported                  bool     `json:"authorization_response_iss_parameter_supported"`
	ClientIDMetadataDocumentSupported      bool     `json:"client_id_metadata_document_supported"`
}

func AuthorizationServerOf(m accessservices.ServerMetadata) AuthorizationServer {
	base := m.Issuer + "/api/oauth/"
	return AuthorizationServer{Issuer: m.Issuer, AuthorizationEndpoint: base + "authorize", TokenEndpoint: base + "token",
		RegistrationEndpoint: base + "register", RevocationEndpoint: base + "revoke",
		ResponseTypesSupported: []string{"code"}, GrantTypesSupported: []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported: []string{"S256"}, TokenEndpointAuthMethodsSupported: []string{"none"},
		RevocationEndpointAuthMethodsSupported: []string{"none"}, ScopesSupported: []string{"mcp"}, IssParameterSupported: true,
		ClientIDMetadataDocumentSupported: m.ClientMetadata}
}

type RegisteredClient struct {
	ClientID                uuid.UUID `json:"client_id"`
	ClientIDIssuedAt        int64     `json:"client_id_issued_at"`
	ClientName              *string   `json:"client_name"`
	RedirectURIs            []string  `json:"redirect_uris"`
	GrantTypes              []string  `json:"grant_types"`
	ResponseTypes           []string  `json:"response_types"`
	TokenEndpointAuthMethod string    `json:"token_endpoint_auth_method"`
}

func RegisteredClientOf(c accesstypes.OAuthClientRecord) RegisteredClient {
	return RegisteredClient{ClientID: c.ID, ClientIDIssuedAt: c.CreatedUnix, ClientName: c.Name, RedirectURIs: c.RedirectURIs,
		GrantTypes: []string{"authorization_code", "refresh_token"}, ResponseTypes: []string{"code"}, TokenEndpointAuthMethod: "none"}
}

type IssuedTokens struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    uint32 `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func IssuedTokensOf(p accesstypes.TokenPair) IssuedTokens {
	return IssuedTokens{AccessToken: p.Access, TokenType: "Bearer", ExpiresIn: p.ExpiresIn, RefreshToken: p.Refresh, Scope: "mcp"}
}

type OAuthFailure struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

type ConsentClient struct {
	ID           uuid.UUID `json:"id"`
	Name         *string   `json:"name"`
	RedirectURI  *string   `json:"redirect_uri,omitempty"`
	RedirectHost string    `json:"redirect_host"`
	MetadataURL  *string   `json:"metadata_url"`
	MetadataHost *string   `json:"metadata_host"`
}

type ConsentRequest struct {
	ID         uuid.UUID     `json:"id"`
	Client     ConsentClient `json:"client"`
	Scopes     []string      `json:"scopes"`
	Resource   *string       `json:"resource"`
	ExpiresAt  string        `json:"expires_at"`
	MCPServers []TokenServer `json:"mcp_servers,omitempty"`
}

func ConsentRequestOf(r accesstypes.AuthorizationRequest) ConsentRequest {
	return ConsentRequest{ID: r.ID, Scopes: r.Scopes.Strings(), Resource: r.Resource, ExpiresAt: r.ExpiresAt,
		Client: ConsentClient{ID: r.ClientID, Name: r.ClientName, RedirectURI: new(r.RedirectURI),
			RedirectHost: accesstypes.RedirectHost(r.RedirectURI), MetadataURL: r.ClientMetadataURL,
			MetadataHost: accesstypes.MetadataHost(r.ClientMetadataURL)},
		MCPServers: consentServers(r.MCPServers)}
}

func consentServers(refs []accesstypes.ServerRef) []TokenServer {
	if refs == nil {
		return nil
	}
	out := make([]TokenServer, len(refs))
	for i, r := range refs {
		out[i] = TokenServer{ID: r.ID, Slug: r.Slug, Title: r.Title, OrganizationID: r.OrganizationID}
	}
	return out
}

type RedirectTo struct {
	RedirectTo string `json:"redirect_to"`
}

type App struct {
	ID         uuid.UUID     `json:"id"`
	Client     ConsentClient `json:"client"`
	Scopes     []string      `json:"scopes"`
	Nodes      []TokenNode   `json:"nodes"`
	CreatedAt  string        `json:"created_at"`
	LastUsedAt *string       `json:"last_used_at"`
	ExpiresAt  string        `json:"expires_at"`
	RevokedAt  *string       `json:"revoked_at"`
	Status     string        `json:"status"`
	MCPServers []TokenServer `json:"mcp_servers"`
}

func AppOf(g accesstypes.AppGrant) App {
	var nodes []TokenNode
	if g.NodeIDs != nil {
		nodes = make([]TokenNode, len(g.Nodes))
		for i, n := range g.Nodes {
			nodes[i] = TokenNode{ID: n.ID, Kind: n.Kind, Name: n.Name, Path: n.Path}
		}
	}
	return App{ID: g.ID, Client: ConsentClient{ID: g.ClientID, Name: g.ClientName, RedirectHost: g.RedirectHost,
		MetadataURL: g.ClientMetadataURL, MetadataHost: accesstypes.MetadataHost(g.ClientMetadataURL)},
		Scopes: g.Scopes.Strings(), Nodes: nodes, CreatedAt: g.CreatedAt, LastUsedAt: g.LastUsedAt, ExpiresAt: g.ExpiresAt,
		RevokedAt: g.RevokedAt, Status: string(g.Status), MCPServers: tokenServers(g.MCPServerSet, g.MCPServers)}
}
