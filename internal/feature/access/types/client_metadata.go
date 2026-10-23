package types

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	ClientMetadataURLMaxLen   = 2000
	ClientMetadataMaxBytes    = 5 << 10
	ClientMetadataTimeout     = 5 * time.Second
	ClientMetadataDefaultLife = time.Hour
	ClientMetadataMaxLife     = 24 * time.Hour
)

const (
	CodeClientMetadataUnavailable = "oauth.client_metadata_unavailable"
	CodeClientMetadataInvalid     = "oauth.client_metadata_invalid"
	CodeClientNotAllowed          = "oauth.client_not_allowed"
	CodeInvalidClient             = "oauth.invalid_client"
	CodeInvalidRedirectURI        = "oauth.invalid_redirect_uri"
)

type FetchedMetadata struct {
	Body         []byte
	CacheControl string
}

type ClientMetadataError struct {
	Code   string
	Detail string
}

func (e *ClientMetadataError) Error() string { return e.Code + ": " + e.Detail }

func metadataInvalid(detail string) *ClientMetadataError {
	return &ClientMetadataError{Code: CodeClientMetadataInvalid, Detail: detail}
}

func ValidClientMetadataURL(raw string) bool {
	if len(raw) > ClientMetadataURLMaxLen || !strings.HasPrefix(raw, "https://") || strings.ContainsAny(raw, "#? \t\r\n\\") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || u.Opaque != "" {
		return false
	}
	path := u.EscapedPath()
	if path == "" || path == "/" {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if s, err := url.PathUnescape(segment); err != nil || s == "." || s == ".." {
			return false
		}
	}
	return true
}

type metadataDocument struct {
	ClientID                *string  `json:"client_id"`
	ClientName              *string  `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod *string  `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	ClientSecret            any      `json:"client_secret"`
	ClientSecretExpiresAt   any      `json:"client_secret_expires_at"`
}

func ParseClientMetadataDocument(clientID string, body []byte) (ClientMetadata, *ClientMetadataError) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return ClientMetadata{}, metadataInvalid("the document is not a JSON object")
	}
	var doc metadataDocument
	if err := json.Unmarshal(trimmed, &doc); err != nil {
		return ClientMetadata{}, metadataInvalid("the document is not valid JSON: " + err.Error())
	}
	switch {
	case doc.ClientID == nil || *doc.ClientID != clientID:
		return ClientMetadata{}, metadataInvalid("client_id of the document differs from its address")
	case doc.ClientName == nil || strings.TrimSpace(*doc.ClientName) == "":
		return ClientMetadata{}, metadataInvalid("client_name is required")
	case doc.ClientSecret != nil || doc.ClientSecretExpiresAt != nil:
		return ClientMetadata{}, metadataInvalid("the document must not carry client_secret")
	}
	meta, refusal := ValidateClientMetadata(ClientMetadata{Name: doc.ClientName, RedirectURIs: doc.RedirectURIs,
		TokenEndpointAuthMethod: doc.TokenEndpointAuthMethod, GrantTypes: doc.GrantTypes, ResponseTypes: doc.ResponseTypes})
	if refusal != nil {
		return ClientMetadata{}, metadataInvalid(refusal.Description)
	}
	return meta, nil
}

func MetadataLifetime(cacheControl string) time.Duration {
	life := ClientMetadataDefaultLife
	for _, directive := range strings.Split(cacheControl, ",") {
		name, value, _ := strings.Cut(strings.TrimSpace(directive), "=")
		switch strings.ToLower(name) {
		case "no-store", "no-cache":
			return 0
		case "max-age":
			secs, err := strconv.ParseUint(strings.Trim(value, `"`), 10, 32)
			if err != nil {
				continue
			}
			life = min(time.Duration(secs)*time.Second, ClientMetadataMaxLife)
		}
	}
	return life
}

func MetadataHost(raw *string) *string {
	if raw == nil {
		return nil
	}
	u, err := url.Parse(*raw)
	if err != nil {
		return nil
	}
	return &u.Host
}
