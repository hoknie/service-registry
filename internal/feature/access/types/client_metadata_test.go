package types

import (
	"strings"
	"testing"
	"time"
)

func TestClientMetadataURLRules(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]bool{
		"https://app.example/oauth/client.json":            true,
		"https://app.example:8443/client":                  true,
		"https://127.0.0.1:4433/client.json":               true,
		"https://app.example":                              false,
		"https://app.example/":                             false,
		"http://app.example/client.json":                   false,
		"HTTPS://app.example/client.json":                  false,
		"https://app.example/client.json#x":                false,
		"https://app.example/client.json?v=1":              false,
		"https://user@app.example/client.json":             false,
		"https://app.example/a/../client.json":             false,
		"https://app.example/a/./client.json":              false,
		"https://app.example/a/%2e%2e/client.json":         false,
		"https://app.example/a b":                          false,
		"https:///client.json":                             false,
		"https://app.example/" + strings.Repeat("a", 2000): false,
	} {
		if got := ValidClientMetadataURL(raw); got != want {
			t.Errorf("%s: got %v", raw, got)
		}
	}
}

func TestParseClientMetadataDocument(t *testing.T) {
	t.Parallel()
	const id = "https://app.example/client.json"
	tests := []struct {
		name string
		body string
		want string
	}{
		{"valid", `{"client_id":"` + id + `","client_name":" Example ","redirect_uris":["http://localhost:3000/cb"],"logo_uri":"https://x"}`, ""},
		{"not an object", `["x"]`, "not a JSON object"},
		{"broken", `{"client_id":`, "not valid JSON"},
		{"other id", `{"client_id":"https://other.example/client.json","client_name":"E","redirect_uris":["http://localhost/cb"]}`, "differs"},
		{"no id", `{"client_name":"E","redirect_uris":["http://localhost/cb"]}`, "differs"},
		{"no name", `{"client_id":"` + id + `","redirect_uris":["http://localhost/cb"]}`, "client_name"},
		{"blank name", `{"client_id":"` + id + `","client_name":"  ","redirect_uris":["http://localhost/cb"]}`, "client_name"},
		{"secret", `{"client_id":"` + id + `","client_name":"E","client_secret":"s","redirect_uris":["http://localhost/cb"]}`, "client_secret"},
		{"no redirects", `{"client_id":"` + id + `","client_name":"E"}`, "redirect_uris"},
		{"unsafe redirect", `{"client_id":"` + id + `","client_name":"E","redirect_uris":["http://evil.example/cb"]}`, "redirect URI"},
		{"private key", `{"client_id":"` + id + `","client_name":"E","redirect_uris":["http://localhost/cb"],"token_endpoint_auth_method":"private_key_jwt"}`, "public clients"},
		{"implicit", `{"client_id":"` + id + `","client_name":"E","redirect_uris":["http://localhost/cb"],"response_types":["token"]}`, "response_types"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			meta, err := ParseClientMetadataDocument(id, []byte(tt.body))
			if tt.want == "" {
				if err != nil || meta.Name == nil || *meta.Name != "Example" || len(meta.RedirectURIs) != 1 {
					t.Fatalf("%+v %v", meta, err)
				}
				return
			}
			if err == nil || err.Code != CodeClientMetadataInvalid || !strings.Contains(err.Detail, tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestMetadataLifetime(t *testing.T) {
	t.Parallel()
	for header, want := range map[string]time.Duration{
		"":                        time.Hour,
		"public":                  time.Hour,
		"max-age=600":             10 * time.Minute,
		"public, Max-Age=\"120\"": 2 * time.Minute,
		"max-age=0":               0,
		"max-age=999999":          24 * time.Hour,
		"max-age=abc":             time.Hour,
		"no-store":                0,
		"max-age=600, no-cache":   0,
	} {
		if got := MetadataLifetime(header); got != want {
			t.Errorf("%q: got %v, want %v", header, got, want)
		}
	}
}

func TestMetadataHost(t *testing.T) {
	t.Parallel()
	if MetadataHost(nil) != nil {
		t.Fatal("nil address has a host")
	}
	raw := "https://app.example:8443/client.json"
	if h := MetadataHost(&raw); h == nil || *h != "app.example:8443" {
		t.Fatalf("host %v", h)
	}
}
