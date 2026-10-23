package types

import (
	"bytes"
	"encoding/json"
	"net/url"
	"slices"
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

const (
	HintMetadataHosts       = "metadata_hosts"
	HintMetadataDisabled    = "metadata_disabled"
	HintMetadataUnavailable = "metadata_unavailable"
	HintMetadataInvalid     = "metadata_invalid"
)

const (
	ReasonTimeout           = "timeout"
	ReasonDNS               = "dns"
	ReasonTLS               = "tls"
	ReasonBlockedAddress    = "blocked_address"
	ReasonConnection        = "connection"
	ReasonRedirect          = "redirect"
	ReasonTooLarge          = "too_large"
	ReasonRateLimited       = "rate_limited"
	ReasonFetchFailed       = "fetch_failed"
	ReasonNotJSON           = "not_json"
	ReasonClientIDMismatch  = "client_id_mismatch"
	ReasonMissingClientName = "missing_client_name"
	ReasonHasSecret         = "has_secret"
	ReasonBadRedirectURI    = "bad_redirect_uri"
	ReasonAuthMethod        = "auth_method"
	ReasonGrantTypes        = "grant_types"
	ReasonResponseTypes     = "response_types"
)

func ReasonStatus(code int) string {
	if code < 100 || code > 599 {
		return ReasonFetchFailed
	}
	return "status_" + strconv.Itoa(code)
}

type FetchedMetadata struct {
	Body         []byte
	CacheControl string
}

type ClientMetadataError struct {
	Code   string
	Reason string
	Detail string
}

func (e *ClientMetadataError) Error() string {
	text := e.Code
	if e.Reason != "" {
		text += " (" + e.Reason + ")"
	}
	if e.Detail != "" {
		text += ": " + e.Detail
	}
	return text
}

func metadataInvalid(reason, detail string) *ClientMetadataError {
	return &ClientMetadataError{Code: CodeClientMetadataInvalid, Reason: reason, Detail: detail}
}

func MetadataUnavailable(reason, detail string) *ClientMetadataError {
	return &ClientMetadataError{Code: CodeClientMetadataUnavailable, Reason: reason, Detail: detail}
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
		return ClientMetadata{}, metadataInvalid(ReasonNotJSON, "the document is not a JSON object")
	}
	var doc metadataDocument
	if err := json.Unmarshal(trimmed, &doc); err != nil {
		return ClientMetadata{}, metadataInvalid(ReasonNotJSON, "the document is not valid JSON: "+err.Error())
	}
	switch {
	case doc.ClientID == nil || *doc.ClientID != clientID:
		return ClientMetadata{}, metadataInvalid(ReasonClientIDMismatch, "client_id of the document differs from its address")
	case doc.ClientName == nil || strings.TrimSpace(*doc.ClientName) == "":
		return ClientMetadata{}, metadataInvalid(ReasonMissingClientName, "client_name is required")
	case doc.ClientSecret != nil || doc.ClientSecretExpiresAt != nil:
		return ClientMetadata{}, metadataInvalid(ReasonHasSecret, "the document must not carry client_secret")
	}
	in := ClientMetadata{Name: doc.ClientName, RedirectURIs: doc.RedirectURIs,
		TokenEndpointAuthMethod: doc.TokenEndpointAuthMethod, GrantTypes: doc.GrantTypes, ResponseTypes: doc.ResponseTypes}
	meta, refusal := ValidateClientMetadata(in)
	if refusal != nil {
		return ClientMetadata{}, metadataInvalid(invalidReason(in, refusal), refusal.Description)
	}
	return meta, nil
}

func invalidReason(in ClientMetadata, refusal *OAuthError) string {
	switch {
	case refusal.Code == "invalid_redirect_uri":
		return ReasonBadRedirectURI
	case in.TokenEndpointAuthMethod != nil && *in.TokenEndpointAuthMethod != "none":
		return ReasonAuthMethod
	case slices.ContainsFunc(in.GrantTypes, func(g string) bool { return g != "authorization_code" && g != "refresh_token" }):
		return ReasonGrantTypes
	default:
		return ReasonResponseTypes
	}
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
