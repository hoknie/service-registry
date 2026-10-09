package tests

import (
	"bytes"
	"crypto/sha256"
	"regexp"
	"strings"
	"testing"
)

const tokensPath = "/api/v1/account/tokens"

func (a *testApp) issue(cookie string, body obj) reply {
	return a.send("POST", tokensPath, cookie, body)
}

func (a *testApp) pat(cookie string, scopes ...string) string {
	a.t.Helper()
	r := a.issue(cookie, obj{"name": "t", "scopes": scopes, "expires_in_days": 30})
	if r.status != 201 {
		a.t.Fatalf("issue: %d %s", r.status, r.body)
	}
	return r.json(a.t)["secret"].(string)
}

func (a *testApp) bearer(method, path, secret string, body any, extra ...string) reply {
	a.t.Helper()
	headers := append([]string{"authorization", "Bearer " + secret}, extra...)
	if body == nil {
		return request(a.t, a.addr, method, path, "", headers...)
	}
	return request(a.t, a.addr, method, path, jsonText(body), append(headers, "content-type", "application/json")...)
}

func code(t testing.TB, r reply) any { return r.json(t)["code"] }

func tokenByID(t testing.TB, r reply, id string) obj {
	t.Helper()
	for _, it := range list(t, r.json(t), "items") {
		if it.(obj)["id"] == id {
			return it.(obj)
		}
	}
	t.Fatalf("token %s not in %s", id, r.body)
	return nil
}

func TestIssuedTokenHasTheDocumentedShapeAndIsShownOnce(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("bob@example.com", false)
	bob := app.session("bob@example.com", password)
	r := app.issue(bob, obj{"name": "  ci  ", "scopes": []string{"write", "read"}, "expires_in_days": 90})
	eq(t, r.status, 201, r.text())
	v := r.json(t)
	secret := v["secret"].(string)
	eq(t, regexp.MustCompile(`^svcp_[0-9A-Za-z]{38}$`).MatchString(secret), true, secret)
	eq(t, v["prefix"], any(secret[:12]))
	eq(t, v["name"], any("ci"))
	eq(t, strings.Join(strs(t, v["scopes"]), ","), "read,write")
	eq(t, v["status"], any("active"))
	eq(t, v["last_used_at"], nil)
	eq(t, v["revoked_at"], nil)
	eq(t, has(v, "user_id"), false)
	eq(t, scalar[bool](t, app.db, "SELECT expires_at BETWEEN created_at + interval '89 days 23 hours' AND created_at + interval '90 days 1 hour' FROM personal_access_tokens"), true)

	stored := scalar[[]byte](t, app.db, "SELECT token_hash FROM personal_access_tokens")
	sum := sha256.Sum256([]byte(secret))
	eq(t, bytes.Equal(stored, sum[:]), true)
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM personal_access_tokens WHERE position(convert_to($1, 'UTF8') IN token_hash) > 0 OR prefix = $1", secret), int64(0))

	listed := app.get(tokensPath, bob)
	eq(t, listed.status, 200)
	lacks(t, listed.text(), "secret")
	lacks(t, listed.text(), secret)
	eq(t, tokenByID(t, listed, v["id"].(string))["status"], any("active"))
}

func TestTokenRulesOnIssue(t *testing.T) {
	t.Parallel()
	app := startApp(t, "PAT_MAX_LIFETIME_DAYS", "365")
	root := app.admin()
	app.user("bob@example.com", false)
	bob := app.session("bob@example.com", password)
	tests := []struct {
		name   string
		cookie string
		body   obj
		status int
		code   string
	}{
		{"unknown scope", bob, obj{"name": "t", "scopes": []string{"read", "deploy"}, "expires_in_days": 1}, 400, "validation.invalid_scopes"},
		{"no scopes", bob, obj{"name": "t", "scopes": []string{}, "expires_in_days": 1}, 400, "validation.invalid_scopes"},
		{"admin of a user", bob, obj{"name": "t", "scopes": []string{"admin"}, "expires_in_days": 1}, 400, "validation.invalid_scopes"},
		{"blank name", bob, obj{"name": " ", "scopes": []string{"read"}, "expires_in_days": 1}, 400, "validation.invalid_token_name"},
		{"no lifetime of a user", bob, obj{"name": "t", "scopes": []string{"read"}}, 400, "validation.invalid_token_lifetime"},
		{"null lifetime of a user", bob, obj{"name": "t", "scopes": []string{"read"}, "expires_in_days": nil}, 400, "validation.invalid_token_lifetime"},
		{"over the maximum", root, obj{"name": "t", "scopes": []string{"read"}, "expires_in_days": 366}, 400, "validation.invalid_token_lifetime"},
		{"zero days", root, obj{"name": "t", "scopes": []string{"read"}, "expires_in_days": 0}, 400, "validation.invalid_token_lifetime"},
		{"no name", bob, obj{"scopes": []string{"read"}, "expires_in_days": 1}, 400, "validation.invalid_body"},
		{"maximum", bob, obj{"name": "t", "scopes": []string{"read"}, "expires_in_days": 365}, 201, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := app.issue(tt.cookie, tt.body)
			eq(t, r.status, tt.status, r.text())
			if tt.code != "" {
				eq(t, code(t, r), any(tt.code))
			}
		})
	}
	eq(t, app.count("personal_access_tokens"), int64(1), "only the valid one is stored")

	r := app.issue(root, obj{"name": "automation", "scopes": []string{"admin"}, "expires_in_days": nil})
	eq(t, r.status, 201, r.text())
	eq(t, r.json(t)["expires_at"], nil)
	eq(t, r.json(t)["status"], any("active"))
	eq(t, app.call("POST", tokensPath, "").status, 401)
}

func TestBearerAuthenticatesAndIgnoresTheCookie(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	app.user("bob@example.com", false)
	ann := app.session("ann@example.com", password)
	secret := app.pat(ann, "mcp")

	r := app.bearer("GET", "/api/v1/auth/me", secret, nil)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["email"], any("ann@example.com"))
	bob := app.session("bob@example.com", password)
	r = app.bearer("GET", "/api/v1/auth/me", secret, nil, "cookie", bob)
	eq(t, r.json(t)["email"], any("ann@example.com"))
	eq(t, request(t, app.addr, "GET", "/api/v1/auth/me", "", "authorization", "bearer "+secret).status, 200)

	_, key := app.projectWithKey(app.admin(), "api")
	bad := []string{
		"svcp_" + strings.Repeat("A", 38),
		secret[:len(secret)-1] + flip(secret[len(secret)-1]),
		key,
	}
	for _, b := range bad {
		r := app.bearer("GET", "/api/v1/auth/me", b, nil, "cookie", ann)
		eq(t, r.status, 401, b)
		eq(t, code(t, r), any("auth.invalid_token"))
		eq(t, r.header("WWW-Authenticate"), "Bearer")
	}
	for _, header := range []string{"Basic YTpi", "Bearer", ""} {
		r := request(t, app.addr, "GET", "/api/v1/auth/me", "", "authorization", header, "cookie", ann)
		eq(t, r.status, 401, header)
		eq(t, code(t, r), any("auth.invalid_token"))
	}
	r = app.get("/api/v1/auth/me", "")
	eq(t, code(t, r), any("auth.unauthenticated"))
	eq(t, request(t, app.addr, "GET", "/api/health", "", "authorization", "Basic x").status, 200)
}

func flip(c byte) string {
	if c == 'a' {
		return "b"
	}
	return "a"
}

func strs(t testing.TB, v any) []string {
	t.Helper()
	var out []string
	for _, s := range v.([]any) {
		out = append(out, s.(string))
	}
	return out
}

func TestExpiredRevokedAndDisabledTokensAreRejected(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	bob := app.user("bob@example.com", false)
	bobCookie := app.session("bob@example.com", password)
	expired := app.pat(bobCookie, "read")
	execSQL(t, app.db, "UPDATE personal_access_tokens SET expires_at = now() - interval '1 minute'")
	r := app.bearer("GET", "/api/v1/auth/me", expired, nil)
	eq(t, code(t, r), any("auth.invalid_token"))
	eq(t, app.get(tokensPath, bobCookie).json(t)["items"].([]any)[0].(obj)["status"], any("expired"))

	live := app.pat(bobCookie, "read")
	eq(t, app.bearer("GET", "/api/v1/auth/me", live, nil).status, 200)
	r = app.send("PATCH", "/api/v1/users/"+bob.ID.String(), root, obj{"status": "disabled"})
	eq(t, r.status, 200, r.text())
	eq(t, code(t, app.bearer("GET", "/api/v1/auth/me", live, nil)), any("auth.invalid_token"))
	statuses := scalar[[]string](t, app.db, "SELECT array_agg(CASE WHEN revoked_at IS NULL THEN 'live' ELSE 'revoked' END) FROM personal_access_tokens")
	eq(t, strings.Join(statuses, ","), "revoked,revoked", "disabling revokes every token")
}

func TestLastUsedIsMarked(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("bob@example.com", false)
	bob := app.session("bob@example.com", password)
	secret := app.pat(bob, "read")
	eq(t, app.bearer("GET", "/api/v1/auth/me", secret, nil).status, 200)
	used := app.get(tokensPath, bob).json(t)["items"].([]any)[0].(obj)
	eq(t, used["last_used_at"] != nil, true)
}

func TestOwnTokensAreListedAndRevoked(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	app.user("bob@example.com", false)
	ann := app.session("ann@example.com", password)
	bob := app.session("bob@example.com", password)
	annSecret := app.pat(ann, "read")
	annID := idOf(app.get(tokensPath, ann).json(t)["items"].([]any)[0].(obj))
	first := app.issue(bob, obj{"name": "first", "scopes": []string{"read"}, "expires_in_days": 1}).json(t)
	second := app.issue(bob, obj{"name": "second", "scopes": []string{"read"}, "expires_in_days": 1}).json(t)

	items := app.get(tokensPath, bob).json(t)["items"].([]any)
	eq(t, len(items), 2)
	eq(t, items[0].(obj)["id"], second["id"], "newest first")

	r := app.call("DELETE", tokensPath+"/"+annID, bob)
	eq(t, r.status, 404)
	eq(t, code(t, r), any("not_found"))
	eq(t, app.bearer("GET", "/api/v1/auth/me", annSecret, nil).status, 200)

	eq(t, app.call("DELETE", tokensPath+"/"+idOf(first), bob).status, 204)
	eq(t, app.call("DELETE", tokensPath+"/"+idOf(first), bob).status, 204, "idempotent")
	eq(t, app.call("DELETE", tokensPath+"/not-a-uuid", bob).status, 404)
	revoked := tokenByID(t, app.get(tokensPath, bob), idOf(first))
	eq(t, revoked["status"], any("revoked"))
	eq(t, revoked["revoked_at"] != nil, true)
	eq(t, code(t, app.bearer("GET", "/api/v1/auth/me", first["secret"].(string), nil)), any("auth.invalid_token"))
	eq(t, app.bearer("GET", "/api/v1/auth/me", second["secret"].(string), nil).status, 200)
}

func TestSuperadminListsSearchesAndRevokesTokens(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	bobUser := app.user("bob@example.com", false)
	bob := app.session("bob@example.com", password)
	secret := app.pat(bob, "read")
	app.pat(root, "admin")

	r := app.get("/api/v1/users/"+bobUser.ID.String()+"/tokens", root)
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	eq(t, len(items), 1)
	eq(t, items[0].(obj)["user_email"], any("bob@example.com"))
	eq(t, items[0].(obj)["user_id"], any(bobUser.ID.String()))
	eq(t, app.get("/api/v1/users/0190a000-0000-7000-8000-000000000000/tokens", root).status, 404)

	all := app.get("/api/v1/tokens", root).json(t)
	eq(t, all["total"], any(float64(2)))
	eq(t, all["limit"], any(float64(50)))
	found := app.get("/api/v1/tokens?prefix="+secret[:9], root).json(t)
	eq(t, found["total"], any(float64(1)), secret[:9])
	eq(t, list(t, found, "items")[0].(obj)["user_email"], any("bob@example.com"))
	eq(t, app.get("/api/v1/tokens?prefix=svcp_%25", root).json(t)["total"], any(float64(0)), "% is literal")
	eq(t, app.get("/api/v1/tokens?limit=1&offset=1", root).json(t)["items"].([]any)[0].(obj)["user_email"], any("bob@example.com"))

	r = app.get("/api/v1/tokens?prefix="+secret, root)
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_token_prefix"))
	eq(t, code(t, app.get("/api/v1/tokens?limit=0", root)), any("validation.invalid_pagination"))

	for _, path := range []string{"/api/v1/tokens", "/api/v1/users/" + bobUser.ID.String() + "/tokens"} {
		r := app.get(path, bob)
		eq(t, r.status, 403, path)
		eq(t, code(t, r), any("auth.forbidden"))
	}
	bobToken := idOf(list(t, found, "items")[0].(obj))
	eq(t, app.call("DELETE", "/api/v1/tokens/"+bobToken, bob).status, 403)
	eq(t, app.call("DELETE", "/api/v1/tokens/"+bobToken, root).status, 204)
	eq(t, app.call("DELETE", "/api/v1/tokens/"+bobToken, root).status, 204)
	eq(t, app.call("DELETE", "/api/v1/tokens/0190a000-0000-7000-8000-000000000000", root).status, 404)
	eq(t, code(t, app.bearer("GET", "/api/v1/auth/me", secret, nil)), any("auth.invalid_token"))
}

func TestTokensNeverManageTokensOrSessions(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	rootUser := scalar[string](t, app.db, "SELECT id::text FROM users WHERE email = 'root@example.com'")
	secret := app.pat(root, "read", "write", "admin", "mcp")
	calls := []struct{ method, path string }{
		{"GET", tokensPath},
		{"POST", tokensPath},
		{"DELETE", tokensPath + "/0190a000-0000-7000-8000-000000000000"},
		{"GET", "/api/v1/users/" + rootUser + "/tokens"},
		{"GET", "/api/v1/tokens"},
		{"DELETE", "/api/v1/tokens/0190a000-0000-7000-8000-000000000000"},
		{"POST", "/api/v1/auth/logout"},
		{"POST", "/api/v1/auth/password"},
	}
	for _, c := range calls {
		r := app.bearer(c.method, c.path, secret, obj{"name": "x", "scopes": []string{"read"}, "expires_in_days": 365})
		eq(t, r.status, 403, c.method+" "+c.path)
		eq(t, code(t, r), any("auth.session_required"), c.path)
	}
	eq(t, app.count("personal_access_tokens"), int64(1))
	eq(t, app.bearer("GET", "/api/v1/auth/me", secret, nil).status, 200, "logout refused, token still works")
	eq(t, code(t, app.bearer("POST", "/api/v1/auth/logout", "nope", nil)), any("auth.invalid_token"))
}

func TestOwnPasswordChangeKeepsTokensAndResetRevokesThem(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	bob := app.user("bob@example.com", false)
	bobCookie := app.session("bob@example.com", password)
	secret := app.pat(bobCookie, "read")
	r := app.send("POST", "/api/v1/auth/password", bobCookie, obj{"current_password": password, "new_password": "another long one"})
	eq(t, r.status, 204, r.text())
	eq(t, app.bearer("GET", "/api/v1/auth/me", secret, nil).status, 200)

	r = app.send("POST", "/api/v1/users/"+bob.ID.String()+"/password", root, obj{"password": "a third long one"})
	eq(t, r.status, 204, r.text())
	eq(t, code(t, app.bearer("GET", "/api/v1/auth/me", secret, nil)), any("auth.invalid_token"))
}

func TestRemovingTheSuperadminFlagRevokesTokensWithoutExpiry(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	ann := app.user("ann@example.com", true)
	annCookie := app.session("ann@example.com", password)
	forever := app.issue(annCookie, obj{"name": "forever", "scopes": []string{"admin"}}).json(t)
	month := app.issue(annCookie, obj{"name": "month", "scopes": []string{"read"}, "expires_in_days": 30}).json(t)

	r := app.send("PATCH", "/api/v1/users/"+ann.ID.String(), root, obj{"is_superadmin": false})
	eq(t, r.status, 200, r.text())
	listed := app.get("/api/v1/users/"+ann.ID.String()+"/tokens", root)
	eq(t, tokenByID(t, listed, idOf(forever))["status"], any("revoked"))
	eq(t, tokenByID(t, listed, idOf(month))["status"], any("active"))

	rootID := scalar[string](t, app.db, "SELECT id::text FROM users WHERE email = 'root@example.com'")
	rootForever := app.issue(root, obj{"name": "f", "scopes": []string{"admin"}}).json(t)
	r = app.send("PATCH", "/api/v1/users/"+rootID, root, obj{"is_superadmin": false})
	eq(t, code(t, r), any("conflict.last_superadmin"))
	eq(t, tokenByID(t, app.get(tokensPath, root), idOf(rootForever))["status"], any("active"))
}

func TestScopesLimitTokenRequests(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	app.user("bob@example.com", false)
	app.user("val@example.com", false)
	app.grant(root, acme, "user", "bob@example.com", "editor")
	app.grant(root, acme, "user", "val@example.com", "viewer")
	bob := app.session("bob@example.com", password)
	val := app.session("val@example.com", password)
	patch := obj{"name": "Acme Corp"}

	read := app.pat(bob, "read")
	eq(t, app.bearer("GET", nodePath(acme), read, nil).json(t)["access"], any("read"))
	r := app.bearer("PATCH", nodePath(acme), read, patch)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	r = app.bearer("DELETE", nodePath("0190a000-0000-7000-8000-000000000000"), read, nil)
	eq(t, code(t, r), any("auth.insufficient_scope"), "scope first: nothing about the node")
	eq(t, code(t, app.bearer("GET", "/api/v1/users", read, nil)), any("auth.insufficient_scope"))

	write := app.pat(bob, "write")
	eq(t, code(t, app.bearer("GET", nodePath(acme), write, nil)), any("auth.insufficient_scope"))
	eq(t, app.bearer("PATCH", nodePath(acme), write, patch).status, 200)

	viewerRW := app.pat(val, "read", "write")
	r = app.bearer("PATCH", nodePath(acme), viewerRW, patch)
	eq(t, code(t, r), any("auth.forbidden"), "never more than the user")

	mcp := app.pat(bob, "mcp")
	eq(t, code(t, app.bearer("GET", "/api/v1/catalog/nodes", mcp, nil)), any("auth.insufficient_scope"))
	eq(t, app.bearer("GET", "/api/v1/auth/me", mcp, nil).status, 200)
}

func TestSuperadminPowersThroughATokenNeedTheAdminScope(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	app.nodeID(root, "organization", "", "acme")

	read := app.pat(root, "read")
	r := app.bearer("GET", "/api/v1/catalog/nodes", read, nil)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["total"], any(float64(0)), "without admin: only bindings")
	eq(t, code(t, app.bearer("GET", "/api/v1/users", read, nil)), any("auth.insufficient_scope"))
	eq(t, code(t, app.bearer("GET", "/api/v1/groups", read, nil)), any("auth.insufficient_scope"))

	admin := app.pat(root, "admin")
	eq(t, app.bearer("GET", "/api/v1/users", admin, nil).status, 200)
	eq(t, app.bearer("GET", "/api/v1/groups", admin, nil).status, 200)
	eq(t, code(t, app.bearer("GET", "/api/v1/catalog/nodes", admin, nil)), any("auth.insufficient_scope"))

	both := app.pat(root, "read", "admin")
	eq(t, app.bearer("GET", "/api/v1/catalog/nodes", both, nil).json(t)["total"], any(float64(1)))
	rw := app.pat(root, "write", "admin")
	r = app.bearer("POST", "/api/v1/catalog/nodes", rw, obj{"kind": "organization", "parent_id": nil, "slug": "beta", "name": "Beta"})
	eq(t, r.status, 201, r.text())
	w := app.pat(root, "write")
	r = app.bearer("POST", "/api/v1/catalog/nodes", w, obj{"kind": "organization", "parent_id": nil, "slug": "gamma", "name": "Gamma"})
	eq(t, code(t, r), any("auth.forbidden"), "top level needs the superadmin")
}

func TestBearerRequestsSkipTheOriginCheckExceptSignIn(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	write := app.pat(root, "write", "admin")
	r := app.bearer("PATCH", nodePath(acme), write, obj{"name": "Acme Corp"}, "sec-fetch-site", "cross-site")
	eq(t, r.status, 200, r.text())
	r = app.bearer("PATCH", nodePath(acme), "svcp_x", obj{"name": "x"}, "sec-fetch-site", "cross-site", "cookie", root)
	eq(t, code(t, r), any("auth.invalid_token"))
	r = app.bearer("POST", "/api/v1/auth/login", write, obj{"email": "root@example.com", "password": password}, "sec-fetch-site", "cross-site")
	eq(t, code(t, r), any("auth.csrf_rejected"))
	r = request(t, app.addr, "PATCH", nodePath(acme), jsonText(obj{"name": "x"}),
		"content-type", "application/json", "cookie", root, "sec-fetch-site", "cross-site")
	eq(t, code(t, r), any("auth.csrf_rejected"))
}

func TestTokenRoutesAnswer405WithAllow(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	r := app.call("PUT", tokensPath, "")
	eq(t, r.status, 405)
	eq(t, r.header("Allow"), "GET,HEAD,POST")
	eq(t, app.call("POST", "/api/v1/tokens", "").header("Allow"), "GET,HEAD")
}

func TestReadTokensReadKeysAndDeployments(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	project, key := app.projectWithKey(root, "api")
	eq(t, app.ingest(project, key, deployed("k-1", "api", "production", "1.0.0", "2026-10-08T10:00:00Z")).status, 201)
	read := app.pat(root, "read", "admin")
	for _, sub := range []string{"keys", "deployments", "environments", "events"} {
		r := app.bearer("GET", nodePath(project)+"/"+sub, read, nil)
		eq(t, r.status, 200, sub+" "+r.text())
	}
	r := app.bearer("POST", nodePath(project)+"/keys", read, nil)
	eq(t, code(t, r), any("auth.insufficient_scope"))
}
