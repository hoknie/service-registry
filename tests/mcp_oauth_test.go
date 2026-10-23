package tests

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"

	"svc-registry/internal/testsupport"
)

const (
	oauthIssuer   = "https://registry.example"
	oauthResource = oauthIssuer + "/api/mcp"
	oauthRedirect = "http://localhost:53682/callback"
)

func oauthApp(t *testing.T, extra ...string) *testApp {
	t.Helper()
	return startApp(t, append([]string{"MCP_OAUTH", "on", "PUBLIC_URL", oauthIssuer}, extra...)...)
}

func (a *testApp) form(path string, values url.Values, headers ...string) reply {
	a.t.Helper()
	return request(a.t, a.addr, "POST", path, values.Encode(),
		append([]string{"content-type", "application/x-www-form-urlencoded"}, headers...)...)
}

func (a *testApp) registerClient(redirects ...string) string {
	a.t.Helper()
	if len(redirects) == 0 {
		redirects = []string{oauthRedirect}
	}
	r := a.send("POST", "/api/oauth/register", "", obj{"client_name": "Claude Code", "redirect_uris": redirects})
	if r.status != 201 {
		a.t.Fatalf("register: %d %s", r.status, r.body)
	}
	return r.json(a.t)["client_id"].(string)
}

func (a *testApp) oauthConfig(clientID string) *oauth2.Config {
	return &oauth2.Config{ClientID: clientID, RedirectURL: oauthRedirect, Scopes: []string{"mcp"},
		Endpoint: oauth2.Endpoint{AuthURL: "http://" + a.addr + "/api/oauth/authorize", TokenURL: "http://" + a.addr + "/api/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams}}
}

type pendingAuth struct {
	cfg       *oauth2.Config
	verifier  string
	requestID string
}

func (a *testApp) startAuthorization(clientID string) pendingAuth {
	a.t.Helper()
	cfg := a.oauthConfig(clientID)
	verifier := oauth2.GenerateVerifier()
	u := cfg.AuthCodeURL("xyz", oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("resource", oauthResource))
	r := request(a.t, a.addr, "GET", strings.TrimPrefix(u, "http://"+a.addr), "")
	if r.status != 302 {
		a.t.Fatalf("authorize: %d %s", r.status, r.body)
	}
	loc, err := url.Parse(r.header("Location"))
	must(a.t, err)
	eq(a.t, loc.Scheme+"://"+loc.Host+loc.Path, oauthIssuer+"/oauth/authorize")
	return pendingAuth{cfg: cfg, verifier: verifier, requestID: loc.Query().Get("request")}
}

func (a *testApp) approve(cookie, requestID string, nodes any) url.Values {
	a.t.Helper()
	r := a.send("POST", "/api/v1/oauth/requests/"+requestID+"/approve", cookie, obj{"nodes": nodes})
	if r.status != 200 {
		a.t.Fatalf("approve: %d %s", r.status, r.body)
	}
	to, err := url.Parse(r.json(a.t)["redirect_to"].(string))
	must(a.t, err)
	eq(a.t, to.Scheme+"://"+to.Host+to.Path, oauthRedirect)
	return to.Query()
}

func (a *testApp) connectApp(cookie string, nodes any) (*oauth2.Token, string) {
	a.t.Helper()
	clientID := a.registerClient()
	p := a.startAuthorization(clientID)
	q := a.approve(cookie, p.requestID, nodes)
	tok, err := p.cfg.Exchange(context.Background(), q.Get("code"), oauth2.VerifierOption(p.verifier),
		oauth2.SetAuthURLParam("resource", oauthResource))
	must(a.t, err)
	return tok, clientID
}

func (a *testApp) mcpStatus(token string) reply {
	a.t.Helper()
	return request(a.t, a.addr, "POST", "/api/mcp", `{"jsonrpc":"2.0","id":1,"method":"ping"}`,
		"authorization", "Bearer "+token, "content-type", "application/json", "accept", "application/json, text/event-stream")
}

func TestMCPOAuthFullFlow(t *testing.T) {
	t.Parallel()
	app := oauthApp(t)
	bob := bobSession(app)

	r := request(t, app.addr, "POST", "/api/mcp", `{}`, "content-type", "application/json")
	eq(t, r.status, 401)
	contains(t, r.header("WWW-Authenticate"), `resource_metadata="`+oauthIssuer+`/.well-known/oauth-protected-resource/api/mcp"`)
	contains(t, r.header("WWW-Authenticate"), `scope="mcp"`)
	bad := app.mcpStatus("svco_nope")
	eq(t, bad.status, 401)
	contains(t, bad.header("WWW-Authenticate"), `error="invalid_token"`)
	narrow := app.mcpStatus(app.pat(bob, "read"))
	eq(t, narrow.status, 403)
	eq(t, narrow.header("WWW-Authenticate"), `Bearer error="insufficient_scope", scope="mcp"`)

	prm := app.get("/.well-known/oauth-protected-resource/api/mcp", "")
	eq(t, prm.status, 200)
	eq(t, prm.header("Cache-Control"), "max-age=3600")
	eq(t, at(prm.json(t), "resource"), any(oauthResource))
	eq(t, jsonText(at(prm.json(t), "authorization_servers")), `["`+oauthIssuer+`"]`)
	eq(t, app.get("/.well-known/oauth-protected-resource", "").status, 200)
	asm := app.get("/.well-known/oauth-authorization-server", "")
	eq(t, asm.status, 200)
	eq(t, jsonText(at(asm.json(t), "code_challenge_methods_supported")), `["S256"]`)
	eq(t, at(asm.json(t), "registration_endpoint"), any(oauthIssuer+"/api/oauth/register"))
	eq(t, at(asm.json(t), "authorization_response_iss_parameter_supported"), any(true))

	clientID := app.registerClient()
	p := app.startAuthorization(clientID)
	req := app.get("/api/v1/oauth/requests/"+p.requestID, bob)
	eq(t, req.status, 200, req.text())
	eq(t, at(req.json(t), "client", "name"), any("Claude Code"))
	eq(t, at(req.json(t), "client", "redirect_host"), any("localhost:53682"))
	eq(t, jsonText(at(req.json(t), "scopes")), `["mcp"]`)

	q := app.approve(bob, p.requestID, nil)
	eq(t, q.Get("state"), "xyz")
	eq(t, q.Get("iss"), oauthIssuer)
	eq(t, app.send("POST", "/api/v1/oauth/requests/"+p.requestID+"/approve", bob, obj{"nodes": nil}).status, 404)

	tok, err := p.cfg.Exchange(context.Background(), q.Get("code"), oauth2.VerifierOption(p.verifier),
		oauth2.SetAuthURLParam("resource", oauthResource))
	must(t, err)
	eq(t, strings.HasPrefix(tok.AccessToken, "svco_") && len(tok.AccessToken) == 43, true, tok.AccessToken)
	eq(t, strings.HasPrefix(tok.RefreshToken, "svcf_") && len(tok.RefreshToken) == 43, true, tok.RefreshToken)
	eq(t, tok.TokenType, "Bearer")

	s := app.mcpSession(tok.AccessToken)
	eq(t, s.InitializeResult().ServerInfo.Name, "svc-registry")
	me := request(t, app.addr, "GET", "/api/v1/auth/me", "", "authorization", "Bearer "+tok.AccessToken)
	eq(t, me.status, 401)
	eq(t, code(t, me), any("auth.invalid_token"))

	refreshed := app.form("/api/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}, "client_id": {clientID}})
	eq(t, refreshed.status, 200, refreshed.text())
	eq(t, refreshed.header("Cache-Control"), "no-store")
	fresh := refreshed.json(t)
	eq(t, app.mcpStatus(fresh["access_token"].(string)).status, 200)
	replay := app.form("/api/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}, "client_id": {clientID}})
	eq(t, replay.status, 400)
	eq(t, at(replay.json(t), "error"), any("invalid_grant"))
	eq(t, app.mcpStatus(fresh["access_token"].(string)).status, 401)
	eq(t, at(app.get("/api/v1/account/apps", bob).json(t), "items", 0, "status"), any("revoked"))
}

func TestMCPOAuthAuthorizeRefusals(t *testing.T) {
	t.Parallel()
	app := oauthApp(t)
	clientID := app.registerClient()
	challenge := oauth2.S256ChallengeFromVerifier(oauth2.GenerateVerifier())
	base := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {oauthRedirect},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "state": {"xyz"}}
	authorize := func(change func(url.Values)) *url.URL {
		v := url.Values{}
		for k, vs := range base {
			v[k] = append([]string{}, vs...)
		}
		change(v)
		r := app.get("/api/oauth/authorize?"+v.Encode(), "")
		eq(t, r.status, 302, r.text())
		loc, err := url.Parse(r.header("Location"))
		must(t, err)
		return loc
	}
	for name, tc := range map[string]struct {
		change func(url.Values)
		code   string
	}{
		"no PKCE":    {func(v url.Values) { v.Del("code_challenge") }, "invalid_request"},
		"plain":      {func(v url.Values) { v.Set("code_challenge_method", "plain") }, "invalid_request"},
		"token type": {func(v url.Values) { v.Set("response_type", "token") }, "unsupported_response_type"},
		"scope":      {func(v url.Values) { v.Set("scope", "read") }, "invalid_scope"},
		"resource":   {func(v url.Values) { v.Set("resource", "https://evil.example/api/mcp") }, "invalid_request"},
	} {
		loc := authorize(tc.change)
		eq(t, loc.Scheme+"://"+loc.Host+loc.Path, oauthRedirect, name)
		eq(t, loc.Query().Get("error"), tc.code, name)
		eq(t, loc.Query().Get("state"), "xyz", name)
		eq(t, loc.Query().Get("iss"), oauthIssuer, name)
	}
	loc := authorize(func(v url.Values) { v.Set("redirect_uri", "http://localhost:53682/other") })
	eq(t, loc.String(), oauthIssuer+"/oauth/authorize?error=oauth.invalid_redirect_uri")
	loc = authorize(func(v url.Values) { v.Set("redirect_uri", "http://localhost:61000/callback") })
	eq(t, loc.Query().Get("request") != "", true, "a loopback redirect may change its port")
	loc = authorize(func(v url.Values) { v.Set("client_id", newID()) })
	eq(t, loc.String(), oauthIssuer+"/oauth/authorize?error=oauth.invalid_client")

	for name, body := range map[string]obj{
		"unsafe":       {"redirect_uris": []string{"http://evil.example/cb"}},
		"confidential": {"redirect_uris": []string{oauthRedirect}, "token_endpoint_auth_method": "client_secret_basic"},
	} {
		r := app.send("POST", "/api/oauth/register", "", body)
		eq(t, r.status, 400, name)
		eq(t, r.json(t)["code"], nil, name)
		eq(t, at(r.json(t), "error"), any(map[string]string{"unsafe": "invalid_redirect_uri", "confidential": "invalid_client_metadata"}[name]), name)
	}
}

func TestMCPOAuthCodesAreSingleUse(t *testing.T) {
	t.Parallel()
	app := oauthApp(t)
	bob := bobSession(app)
	clientID := app.registerClient()

	p := app.startAuthorization(clientID)
	q := app.approve(bob, p.requestID, nil)
	wrong := app.form("/api/oauth/token", url.Values{"grant_type": {"authorization_code"}, "code": {q.Get("code")},
		"redirect_uri": {oauthRedirect}, "client_id": {clientID}, "code_verifier": {oauth2.GenerateVerifier()}})
	eq(t, wrong.status, 400)
	eq(t, at(wrong.json(t), "error"), any("invalid_grant"))

	p = app.startAuthorization(clientID)
	q = app.approve(bob, p.requestID, nil)
	exchange := url.Values{"grant_type": {"authorization_code"}, "code": {q.Get("code")}, "redirect_uri": {oauthRedirect},
		"client_id": {clientID}, "code_verifier": {p.verifier}}
	first := app.form("/api/oauth/token", exchange)
	eq(t, first.status, 200, first.text())
	access := first.json(t)["access_token"].(string)
	eq(t, app.mcpStatus(access).status, 200)
	again := app.form("/api/oauth/token", exchange)
	eq(t, again.status, 400)
	eq(t, at(again.json(t), "error"), any("invalid_grant"))
	eq(t, app.mcpStatus(access).status, 401)

	unknown := app.form("/api/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"svcf_x"}, "client_id": {newID()}})
	eq(t, unknown.status, 401)
	eq(t, at(unknown.json(t), "error"), any("invalid_client"))
	password := app.form("/api/oauth/token", url.Values{"grant_type": {"password"}}, "sec-fetch-site", "cross-site")
	eq(t, password.status, 400)
	eq(t, at(password.json(t), "error"), any("unsupported_grant_type"))
	eq(t, password.json(t)["code"], nil)
	eq(t, password.header("Cache-Control"), "no-store")
	notAllowed := app.get("/api/oauth/token", "")
	eq(t, notAllowed.status, 405)
	eq(t, notAllowed.header("Allow"), "POST")
}

func TestMCPOAuthAppsAreListedAndRevoked(t *testing.T) {
	t.Parallel()
	app := oauthApp(t)
	admin := app.admin()
	bobUser := app.user("bob@example.com", false)
	bob := app.session("bob@example.com", password)

	tok, clientID := app.connectApp(bob, nil)
	revoked := app.form("/api/oauth/revoke", url.Values{"token": {tok.RefreshToken}, "client_id": {clientID}})
	eq(t, revoked.status, 200)
	eq(t, app.form("/api/oauth/revoke", url.Values{"token": {"svcf_unknown"}, "client_id": {clientID}}).status, 200)
	eq(t, app.mcpStatus(tok.AccessToken).status, 401)

	tok, _ = app.connectApp(bob, nil)
	apps := app.get("/api/v1/account/apps", bob)
	eq(t, apps.status, 200)
	items := apps.json(t)["items"].([]any)
	eq(t, len(items), 2)
	eq(t, at(items[0], "status"), any("active"))
	eq(t, at(items[0], "client", "name"), any("Claude Code"))
	eq(t, at(items[0], "nodes"), nil)
	eq(t, at(items[1], "status"), any("revoked"))
	id := at(items[0], "id").(string)

	pat := app.pat(bob, "read")
	byToken := request(t, app.addr, "GET", "/api/v1/account/apps", "", "authorization", "Bearer "+pat)
	eq(t, code(t, byToken), any("auth.session_required"))
	p := app.startAuthorization(app.registerClient())
	approveByToken := request(t, app.addr, "POST", "/api/v1/oauth/requests/"+p.requestID+"/approve", `{"nodes":null}`,
		"authorization", "Bearer "+pat, "content-type", "application/json")
	eq(t, approveByToken.status, 403)
	eq(t, code(t, approveByToken), any("auth.session_required"))
	deny := app.send("POST", "/api/v1/oauth/requests/"+p.requestID+"/deny", bob, obj{})
	eq(t, deny.status, 200, deny.text())
	contains(t, at(deny.json(t), "redirect_to").(string), "error=access_denied")
	contains(t, at(deny.json(t), "redirect_to").(string), "state=xyz")

	userApps := "/api/v1/users/" + bobUser.ID.String() + "/apps"
	eq(t, app.get(userApps, bob).status, 403)
	adminPat := app.pat(admin, "read", "users:read")
	eq(t, code(t, request(t, app.addr, "GET", userApps, "", "authorization", "Bearer "+adminPat)), any("auth.session_required"))
	eq(t, len(app.get(userApps, admin).json(t)["items"].([]any)), 2)
	eq(t, app.call("DELETE", "/api/v1/account/apps/"+id, admin).status, 404)
	eq(t, app.call("DELETE", "/api/v1/account/apps/"+id, bob).status, 204)
	eq(t, app.call("DELETE", "/api/v1/account/apps/"+id, bob).status, 204)
	eq(t, app.mcpStatus(tok.AccessToken).status, 401)
	stale := app.form("/api/oauth/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}, "client_id": {clientID}})
	eq(t, stale.status, 400)

	tok, _ = app.connectApp(bob, nil)
	eq(t, app.send("PATCH", "/api/v1/users/"+bobUser.ID.String(), admin, obj{"status": "disabled"}).status, 200)
	for _, item := range app.get(userApps, admin).json(t)["items"].([]any) {
		eq(t, at(item, "status"), any("revoked"))
	}
	eq(t, app.mcpStatus(tok.AccessToken).status, 401)
}

func TestMCPOAuthGrantNodesNarrowTheTools(t *testing.T) {
	t.Parallel()
	app := oauthApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	app.grant(tr.admin, tr.acme, "user", "bob@example.com", "viewer")
	app.nodeID(tr.admin, "project", tr.acme, "web")

	r := app.send("POST", "/api/v1/oauth/requests/"+app.startAuthorization(app.registerClient()).requestID+"/approve", bob,
		obj{"nodes": []string{tr.globex}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_token_nodes"))

	tok, _ := app.connectApp(bob, []string{tr.backend})
	out, isErr := call(t, app.mcpSession(tok.AccessToken), "list_projects", map[string]any{})
	eq(t, isErr, false, out)
	contains(t, out, "acme/backend/api")
	eq(t, strings.Contains(out, "acme/web"), false, out)
	eq(t, at(app.get("/api/v1/account/apps", bob).json(t), "items", 0, "nodes", 0, "path"), any("acme/backend"))
}

func TestMCPOAuthIsOffByDefault(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	for _, path := range []string{"/.well-known/oauth-authorization-server", "/.well-known/oauth-protected-resource/api/mcp"} {
		r := app.get(path, "")
		eq(t, r.status, 404, path)
		eq(t, code(t, r), any("not_found"), path)
	}
	r := app.send("POST", "/api/oauth/register", "", obj{"redirect_uris": []string{oauthRedirect}})
	eq(t, r.status, 404)
	eq(t, code(t, r), any("not_found"))
	eq(t, code(t, app.get("/api/v1/oauth/requests/"+newID(), app.admin())), any("not_found"))
	eq(t, request(t, app.addr, "POST", "/api/mcp", `{}`).header("WWW-Authenticate"), "Bearer")
	eq(t, app.get("/api/v1/account/apps", app.session("root@example.com", password)).status, 200)
}

func TestMCPOAuthTokensStopWhenTurnedOff(t *testing.T) {
	t.Parallel()
	app := oauthApp(t)
	bob := bobSession(app)
	tok, _ := app.connectApp(bob, nil)
	eq(t, app.mcpStatus(tok.AccessToken).status, 200)
	app.restart("MCP_OAUTH", "off")
	r := app.mcpStatus(tok.AccessToken)
	eq(t, r.status, 401)
	eq(t, code(t, r), any("auth.invalid_token"))
	eq(t, len(app.get("/api/v1/account/apps", app.session("bob@example.com", password)).json(t)["items"].([]any)), 1)
}

func TestMCPOAuthRegistrationsAreLimited(t *testing.T) {
	t.Parallel()
	app := oauthApp(t)
	for range 10 {
		app.registerClient()
	}
	r := app.send("POST", "/api/oauth/register", "", obj{"redirect_uris": []string{oauthRedirect}})
	eq(t, r.status, 429)
	eq(t, at(r.json(t), "error"), any("rate_limited"))
	eq(t, r.header("Retry-After") != "", true)
}

func TestMCPOAuthAbandonedClientsArePruned(t *testing.T) {
	t.Parallel()
	app := oauthApp(t)
	clientID := app.registerClient()
	kept := app.registerClient()
	execSQL(t, app.db, "UPDATE oauth_clients SET created_at = now() - interval '31 days'")
	execSQL(t, app.db, "UPDATE oauth_clients SET created_at = now() WHERE id = $1", kept)
	_, err := app.services.Access.AuthorizationServer.Prune(context.Background())
	must(t, err)
	v := url.Values{"response_type": {"code"}, "client_id": {clientID}, "redirect_uri": {oauthRedirect}}
	eq(t, app.get("/api/oauth/authorize?"+v.Encode(), "").header("Location"), oauthIssuer+"/oauth/authorize?error=oauth.invalid_client")
	eq(t, app.count("oauth_clients"), int64(1))
	app.user("bob@example.com", false)
	contains(t, execErr(t, app.db, "INSERT INTO oauth_grants (id, user_id, client_id, scopes, refresh_hash, refresh_expires_at) "+
		"SELECT $1, id, $2, '{read}', $3, now() FROM users LIMIT 1", newID(), kept, []byte(strings.Repeat("h", 32))), "oauth_grants_scopes_check")
}

func TestMCPOAuthMetadataIsNotServedFromTheExport(t *testing.T) {
	t.Parallel()
	tdb := testsupport.NewTestDB(t)
	migrate(t, tdb)
	dist := testsupport.ExportDist(t)
	dist.Write(t, ".well-known/oauth-authorization-server", "a file")
	addr := spawnApp(t, tdb.URL, dist.Dir, "MCP_OAUTH", "on", "PUBLIC_URL", oauthIssuer+"/p")
	r := request(t, addr, "GET", "/.well-known/oauth-authorization-server/p", "")
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "issuer"), any(oauthIssuer+"/p"))
	eq(t, at(request(t, addr, "GET", "/.well-known/oauth-authorization-server", "").json(t), "token_endpoint"), any(oauthIssuer+"/p/api/oauth/token"))
	eq(t, request(t, addr, "POST", "/.well-known/oauth-authorization-server", "").status, 405)

	page := request(t, addr, "GET", "/en/login", "")
	eq(t, page.header("X-Frame-Options"), "DENY")
	moved := request(t, addr, "GET", "/oauth/authorize?request=abc", "", "accept-language", "ru")
	eq(t, moved.status, 307)
	eq(t, moved.header("Location"), "/ru/oauth/authorize?request=abc")
}
