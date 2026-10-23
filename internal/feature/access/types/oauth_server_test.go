package types

import (
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestRedirectURIRules(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]bool{
		"https://app.example/cb":          true,
		"http://localhost:53682/callback": true,
		"http://127.0.0.1/cb":             true,
		"http://[::1]:8080/cb":            true,
		"com.example.app:/oauth":          true,
		"cursor://anysphere.cursor/cb":    true,
		"http://evil.example/cb":          false,
		"https://app.example/cb#frag":     false,
		"javascript:alert(1)":             false,
		"data:text/html,x":                false,
		"file:///etc/passwd":              false,
		"vbscript:x":                      false,
		"https://user:pw@app.example/cb":  false,
		"https:///cb":                     false,
		"":                                false,
		"https://app.example/" + strings.Repeat("a", 2000): false,
		"my-app1://cb": false,
	} {
		if got := ValidRedirectURI(raw); got != want {
			t.Errorf("%q: got %v, want %v", raw, got, want)
		}
	}
}

func TestRedirectMatchesExactOrLoopbackWithAnyPort(t *testing.T) {
	t.Parallel()
	registered := []string{"http://localhost:53682/callback", "https://app.example/cb"}
	for presented, want := range map[string]bool{
		"http://localhost:53682/callback": true,
		"http://localhost:61000/callback": true,
		"http://localhost/callback":       true,
		"http://localhost:53682/other":    false,
		"http://127.0.0.1:53682/callback": false,
		"https://app.example/cb":          true,
		"https://app.example:8443/cb":     false,
		"https://app.example/cb?x=1":      false,
	} {
		if got := RedirectMatches(registered, presented); got != want {
			t.Errorf("%q: got %v, want %v", presented, got, want)
		}
	}
}

func TestPKCEVerifier(t *testing.T) {
	t.Parallel()
	verifier := oauth2.GenerateVerifier()
	challenge := oauth2.S256ChallengeFromVerifier(verifier)
	if !ValidCodeChallenge(challenge) || !VerifierMatches(verifier, challenge) {
		t.Fatal("a generated verifier must match its challenge")
	}
	for _, v := range []string{verifier[:42], strings.Repeat("a", 129), verifier + "!", oauth2.GenerateVerifier()} {
		if VerifierMatches(v, challenge) {
			t.Errorf("verifier %q matched", v)
		}
	}
	if VerifierMatches(verifier, verifier) {
		t.Error("plain challenge accepted")
	}
	if ValidCodeChallenge("short") {
		t.Error("short challenge accepted")
	}
}

func TestClientMetadataRules(t *testing.T) {
	t.Parallel()
	ok := []string{"http://localhost:1/cb"}
	long := strings.Repeat("я", 150)
	out, err := ValidateClientMetadata(ClientMetadata{Name: &long, RedirectURIs: ok})
	if err != nil || len([]rune(*out.Name)) != OAuthClientNameMaxLen {
		t.Fatalf("%+v %v", out, err)
	}
	basic := "client_secret_basic"
	for name, tc := range map[string]struct {
		in   ClientMetadata
		code string
	}{
		"no redirects": {ClientMetadata{}, "invalid_redirect_uri"},
		"too many":     {ClientMetadata{RedirectURIs: make([]string, 11)}, "invalid_redirect_uri"},
		"unsafe":       {ClientMetadata{RedirectURIs: []string{"http://evil.example/cb"}}, "invalid_redirect_uri"},
		"confidential": {ClientMetadata{RedirectURIs: ok, TokenEndpointAuthMethod: &basic}, "invalid_client_metadata"},
		"implicit":     {ClientMetadata{RedirectURIs: ok, GrantTypes: []string{"implicit"}}, "invalid_client_metadata"},
		"token type":   {ClientMetadata{RedirectURIs: ok, ResponseTypes: []string{"token"}}, "invalid_client_metadata"},
	} {
		if _, err := ValidateClientMetadata(tc.in); err == nil || err.Code != tc.code {
			t.Errorf("%s: %v, want %s", name, err, tc.code)
		}
	}
}
