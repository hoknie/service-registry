package tests

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"golang.org/x/oauth2"

	svcapp "svc-registry/internal/app"
	"svc-registry/internal/feature/access/clients/clientmeta"
	"svc-registry/pkg/outbound"
)

type documentServer struct {
	srv  *httptest.Server
	hits atomic.Int64
	opt  svcapp.Option
}

func newDocumentServer(t *testing.T) *documentServer {
	t.Helper()
	d := &documentServer{}
	d.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.hits.Add(1)
		doc := map[string]any{"client_id": "https://" + r.Host + r.URL.Path, "client_name": "Example MCP Client",
			"redirect_uris": []string{oauthRedirect}, "token_endpoint_auth_method": "none"}
		switch r.URL.Path {
		case "/client.json":
			w.Header().Set("Cache-Control", "max-age=600")
		case "/nostore.json":
			w.Header().Set("Cache-Control", "no-store")
		case "/other.json":
			doc["client_id"] = "https://other.example/client.json"
		case "/secret.json":
			doc["client_secret"] = "s"
		case "/redirect.json":
			http.Redirect(w, r, "/client.json", http.StatusFound)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(doc)
	}))
	t.Cleanup(d.srv.Close)
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: d.srv.Certificate().Raw})
	d.opt = svcapp.WithClientMetadata(clientmeta.NewWithPolicy(outbound.Options{CAPEM: ca}, anyIP{}))
	return d
}

func (d *documentServer) url(path string) string { return d.srv.URL + path }

func documentApp(t *testing.T, d *documentServer, extra ...string) *testApp {
	t.Helper()
	return startAppWith(t, []svcapp.Option{d.opt}, append([]string{"MCP_OAUTH", "on", "PUBLIC_URL", oauthIssuer,
		"MCP_OAUTH_CLIENT_METADATA_HOSTS", "127.0.0.1"}, extra...)...)
}

func (a *testApp) authorizeRefusal(clientID, redirect string) string {
	a.t.Helper()
	return a.authorizeRefusalAs(clientID, redirect, "").Get("error")
}

func (a *testApp) authorizeRefusalAs(clientID, redirect, session string) url.Values {
	a.t.Helper()
	v := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {redirect},
		"code_challenge": {oauth2.S256ChallengeFromVerifier(oauth2.GenerateVerifier())}, "code_challenge_method": {"S256"}}
	r := a.get("/api/oauth/authorize?"+v.Encode(), session)
	eq(a.t, r.status, 302, r.text())
	loc, err := url.Parse(r.header("Location"))
	must(a.t, err)
	eq(a.t, loc.Scheme+"://"+loc.Host+loc.Path, oauthIssuer+"/oauth/authorize")
	return loc.Query()
}

func TestMCPOAuthClientMetadataDocumentFlow(t *testing.T) {
	t.Parallel()
	docs := newDocumentServer(t)
	app := documentApp(t, docs)
	bob := bobSession(app)
	clientID := docs.url("/client.json")
	host := strings.TrimPrefix(docs.srv.URL, "https://")

	asm := app.get("/.well-known/oauth-authorization-server", "")
	eq(t, at(asm.json(t), "client_id_metadata_document_supported"), any(true))

	p := app.startAuthorization(clientID)
	eq(t, docs.hits.Load(), int64(1))
	req := app.get("/api/v1/oauth/requests/"+p.requestID, bob)
	eq(t, req.status, 200, req.text())
	eq(t, at(req.json(t), "client", "name"), any("Example MCP Client"))
	eq(t, at(req.json(t), "client", "metadata_url"), any(clientID))
	eq(t, at(req.json(t), "client", "metadata_host"), any(host))
	internalID := at(req.json(t), "client", "id").(string)

	q := app.approve(bob, p.requestID, nil)
	tok, err := p.cfg.Exchange(context.Background(), q.Get("code"), oauth2.VerifierOption(p.verifier),
		oauth2.SetAuthURLParam("resource", oauthResource))
	must(t, err)
	eq(t, app.mcpStatus(tok.AccessToken).status, 200)
	refreshed := app.form("/api/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}, "client_id": {clientID}})
	eq(t, refreshed.status, 200, refreshed.text())

	again := app.startAuthorization(clientID)
	eq(t, docs.hits.Load(), int64(1), "the cached document is not fetched again")
	q = app.approve(bob, again.requestID, nil)
	byUUID := app.form("/api/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {q.Get("code")},
		"redirect_uri": {oauthRedirect}, "client_id": {internalID}, "code_verifier": {again.verifier}})
	eq(t, byUUID.status, 401, byUUID.text())
	eq(t, at(byUUID.json(t), "error"), any("invalid_client"))
	eq(t, app.authorizeRefusal(internalID, oauthRedirect), "oauth.invalid_client")

	apps := app.get("/api/v1/account/apps", bob)
	eq(t, at(apps.json(t), "items", 0, "client", "metadata_host"), any(host))
	revoked := app.form("/api/oauth/revoke", url.Values{"token": {fresh(t, refreshed)}, "client_id": {clientID}})
	eq(t, revoked.status, 200)
	eq(t, at(app.get("/api/v1/account/apps", bob).json(t), "items", 0, "status"), any("revoked"))
}

func fresh(t *testing.T, r reply) string {
	t.Helper()
	return r.json(t)["refresh_token"].(string)
}

func TestMCPOAuthClientMetadataRefusals(t *testing.T) {
	t.Parallel()
	docs := newDocumentServer(t)
	app := documentApp(t, docs)
	for name, tc := range map[string]struct {
		path, redirect, code string
	}{
		"other client_id":      {"/other.json", oauthRedirect, "oauth.client_metadata_invalid"},
		"redirect":             {"/redirect.json", oauthRedirect, "oauth.client_metadata_unavailable"},
		"missing":              {"/missing.json", oauthRedirect, "oauth.client_metadata_unavailable"},
		"foreign redirect":     {"/client.json", "http://localhost:9999/elsewhere", "oauth.invalid_redirect_uri"},
		"address with query":   {"/client.json?x=1", oauthRedirect, "oauth.invalid_client"},
		"address without path": {"", oauthRedirect, "oauth.invalid_client"},
	} {
		eq(t, app.authorizeRefusal(docs.url(tc.path), tc.redirect), tc.code, name)
	}
	eq(t, app.form("/api/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"svcf_x"},
		"client_id": {docs.url("/unknown.json")}}).status, 401)

	before := docs.hits.Load()
	app.startAuthorization(docs.url("/nostore.json"))
	app.startAuthorization(docs.url("/nostore.json"))
	eq(t, docs.hits.Load()-before, int64(2), "no-store documents are fetched on every authorization")
}

func TestMCPOAuthClientMetadataPolicy(t *testing.T) {
	t.Parallel()
	docs := newDocumentServer(t)

	guarded := startApp(t, "MCP_OAUTH", "on", "PUBLIC_URL", oauthIssuer, "MCP_OAUTH_CLIENT_METADATA_HOSTS", "127.0.0.1")
	eq(t, guarded.authorizeRefusal(docs.url("/client.json"), oauthRedirect), "oauth.client_metadata_unavailable")
	eq(t, docs.hits.Load(), int64(0), "a loopback address is never dialled")

	unlisted := documentApp(t, docs, "MCP_OAUTH_CLIENT_METADATA_HOSTS", "")
	eq(t, unlisted.authorizeRefusal(docs.url("/client.json"), oauthRedirect), "oauth.client_not_allowed")
	eq(t, at(unlisted.get("/.well-known/oauth-authorization-server", "").json(t), "client_id_metadata_document_supported"), any(false))

	listed := documentApp(t, docs, "MCP_OAUTH_CLIENT_METADATA_HOSTS", "*.trusted.example")
	eq(t, listed.authorizeRefusal(docs.url("/client.json"), oauthRedirect), "oauth.client_not_allowed")

	off := documentApp(t, docs, "MCP_OAUTH_CLIENT_METADATA", "off")
	eq(t, off.authorizeRefusal(docs.url("/client.json"), oauthRedirect), "oauth.invalid_client")
	eq(t, at(off.get("/.well-known/oauth-authorization-server", "").json(t), "client_id_metadata_document_supported"), any(false))
	eq(t, docs.hits.Load(), int64(0))
	off.registerClient()
}

func TestMCPOAuthClientMetadataRefusalHintsForSuperadmins(t *testing.T) {
	t.Parallel()
	docs := newDocumentServer(t)
	clientID := "https://app.example/client.json"

	listed := documentApp(t, docs, "MCP_OAUTH_CLIENT_METADATA_HOSTS", "*.trusted.example")
	root := listed.admin()
	q := listed.authorizeRefusalAs(clientID, oauthRedirect, root)
	eq(t, q.Get("error"), "oauth.client_not_allowed")
	eq(t, q.Get("hint"), "metadata_hosts")
	eq(t, q.Get("host"), "app.example")
	q = listed.authorizeRefusalAs(clientID, oauthRedirect, bobSession(listed))
	eq(t, q.Get("error"), "oauth.client_not_allowed")
	eq(t, q.Get("hint")+q.Get("host"), "", "other users get no details")
	eq(t, listed.authorizeRefusalAs(clientID, oauthRedirect, "").Get("host"), "")

	off := documentApp(t, docs, "MCP_OAUTH_CLIENT_METADATA", "off")
	offRoot := off.admin()
	q = off.authorizeRefusalAs(clientID, oauthRedirect, offRoot)
	eq(t, q.Get("error"), "oauth.invalid_client")
	eq(t, q.Get("hint"), "metadata_disabled")
	eq(t, q.Get("host"), "app.example")
	eq(t, off.authorizeRefusalAs(newID(), oauthRedirect, offRoot).Get("hint"), "", "an unknown registered client gets no hint")

	eq(t, docs.hits.Load(), int64(0))
}

func TestMCPOAuthClientMetadataRefusalReasonsForSuperadmins(t *testing.T) {
	t.Parallel()
	docs := newDocumentServer(t)
	app := documentApp(t, docs)
	root, bob := app.admin(), bobSession(app)
	for path, want := range map[string][3]string{
		"/missing.json":  {"oauth.client_metadata_unavailable", "metadata_unavailable", "status_404"},
		"/redirect.json": {"oauth.client_metadata_unavailable", "metadata_unavailable", "redirect"},
		"/other.json":    {"oauth.client_metadata_invalid", "metadata_invalid", "client_id_mismatch"},
		"/secret.json":   {"oauth.client_metadata_invalid", "metadata_invalid", "has_secret"},
	} {
		q := app.authorizeRefusalAs(docs.url(path), oauthRedirect, root)
		eq(t, [3]string{q.Get("error"), q.Get("hint"), q.Get("reason")}, want, path)
		eq(t, q.Get("host"), "127.0.0.1", path)
		q = app.authorizeRefusalAs(docs.url(path), oauthRedirect, bob)
		eq(t, q.Get("error"), want[0], path)
		eq(t, q.Get("hint")+q.Get("reason")+q.Get("host"), "", "other users get no reason")
		eq(t, app.authorizeRefusalAs(docs.url(path), oauthRedirect, "").Get("reason"), "", "no session, no reason")
	}

	guarded := startApp(t, "MCP_OAUTH", "on", "PUBLIC_URL", oauthIssuer, "MCP_OAUTH_CLIENT_METADATA_HOSTS", "127.0.0.1")
	q := guarded.authorizeRefusalAs(docs.url("/client.json"), oauthRedirect, guarded.admin())
	eq(t, q.Get("error"), "oauth.client_metadata_unavailable")
	eq(t, q.Get("reason"), "blocked_address")
}

func TestMCPOAuthClientMetadataAnyHost(t *testing.T) {
	t.Parallel()
	docs := newDocumentServer(t)

	guarded := startApp(t, "MCP_OAUTH", "on", "PUBLIC_URL", oauthIssuer, "MCP_OAUTH_CLIENT_METADATA_HOSTS", "*")
	eq(t, at(guarded.get("/.well-known/oauth-authorization-server", "").json(t), "client_id_metadata_document_supported"), any(true))
	eq(t, guarded.authorizeRefusal(docs.url("/client.json"), oauthRedirect), "oauth.client_metadata_unavailable")
	eq(t, docs.hits.Load(), int64(0), "* keeps the address policy: loopback is never dialled")

	wide := documentApp(t, docs, "MCP_OAUTH_CLIENT_METADATA_HOSTS", "*")
	wide.startAuthorization(docs.url("/client.json"))
	eq(t, docs.hits.Load(), int64(1))
}
