package tests

import (
	"bytes"
	"crypto/sha256"
	"strings"
	"testing"
)

func TestCookieAttributesByDefault(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	set := app.login("ann@example.com", password).header("Set-Cookie")
	attrs := strings.Split(set, "; ")
	for _, want := range []string{"Path=/", "HttpOnly", "SameSite=Lax", "Secure", "Max-Age=604800"} {
		found := false
		for _, a := range attrs {
			found = found || a == want
		}
		eq(t, found, true, want+" in "+set)
	}
	pair, _, _ := strings.Cut(set, ";")
	token := tokenOf(pair)
	eq(t, len(token), 43, "32 random bytes, base64url without padding")
	for _, c := range token {
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_'
		eq(t, ok, true, string(c))
	}
}

func TestCookieWithoutSecureForLocalHTTP(t *testing.T) {
	t.Parallel()
	app := startApp(t, "SESSION_COOKIE_SECURE", "false")
	app.user("ann@example.com", false)
	set := app.login("ann@example.com", password).header("Set-Cookie")
	lacks(t, set, "Secure")
	contains(t, set, "HttpOnly")
}

func TestOnlyTheTokenHashIsStored(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	token := tokenOf(app.session("ann@example.com", password))
	stored := scalar[[]byte](t, app.db, "SELECT token_hash FROM sessions")
	sum := sha256.Sum256([]byte(token))
	eq(t, bytes.Equal(stored, sum[:]), true)
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM sessions WHERE position(convert_to($1, 'UTF8') IN token_hash) > 0", token), int64(0))
}

func TestIdleTimeoutExpiresTheSession(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	cookie := app.session("ann@example.com", password)
	execSQL(t, app.db, "UPDATE sessions SET last_seen_at = now() - interval '13 hours'")
	r := app.get("/api/v1/auth/me", cookie)
	eq(t, r.status, 401)
	eq(t, r.json(t)["code"], any("auth.unauthenticated"))
}

func TestAbsoluteTimeoutExpiresASessionInUse(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	cookie := app.session("ann@example.com", password)
	execSQL(t, app.db, "UPDATE sessions SET created_at = now() - interval '8 days', "+
		"expires_at = now() - interval '1 day', last_seen_at = now() - interval '1 minute'")
	eq(t, app.get("/api/v1/auth/me", cookie).status, 401)
}

func TestActivityExtendsTheIdleTimer(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	cookie := app.session("ann@example.com", password)
	execSQL(t, app.db, "UPDATE sessions SET last_seen_at = now() - interval '2 hours'")
	eq(t, app.get("/api/v1/auth/me", cookie).status, 200)
	eq(t, scalar[bool](t, app.db, "SELECT last_seen_at > now() - interval '1 minute' FROM sessions"), true, "last_seen_at was refreshed")
}

func TestLogoutRevokesAndClearsTheCookie(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	cookie := app.session("ann@example.com", password)
	r := app.call("POST", "/api/v1/auth/logout", cookie)
	eq(t, r.status, 204)
	set := r.header("Set-Cookie")
	eq(t, strings.HasPrefix(set, "registry_session=;") && strings.Contains(set, "Max-Age=0"), true, set)
	eq(t, app.get("/api/v1/auth/me", cookie).status, 401)
	eq(t, app.call("POST", "/api/v1/auth/logout", "").status, 204, "logout without a session")
	eq(t, app.call("POST", "/api/v1/auth/logout", "registry_session=gone").status, 204)
}

func TestCrossOriginStateChangeIsRejected(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	cookie := app.session("ann@example.com", password)

	r := request(t, app.addr, "POST", "/api/v1/auth/logout", "", "cookie", cookie, "origin", "https://evil.example")
	eq(t, r.status, 403)
	eq(t, r.json(t)["code"], any("auth.csrf_rejected"))
	eq(t, app.get("/api/v1/auth/me", cookie).status, 200, "session not revoked")

	eq(t, request(t, app.addr, "POST", "/api/v1/auth/logout", "", "cookie", cookie, "origin", "null").status, 403)

	body := `{"email":"ann@example.com","password":"` + password + `"}`
	r = request(t, app.addr, "POST", "/api/v1/auth/login", body,
		"content-type", "application/json", "sec-fetch-site", "cross-site", "origin", "http://"+app.addr)
	eq(t, r.status, 403, "Fetch Metadata wins over Origin")
	eq(t, r.hasHeader("Set-Cookie"), false)
}

func TestSameOriginAndNonBrowserRequestsPass(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("ann@example.com", false)
	origin := "http://" + app.addr
	body := `{"email":"ann@example.com","password":"` + password + `"}`
	eq(t, request(t, app.addr, "POST", "/api/v1/auth/login", body,
		"content-type", "application/json", "origin", origin, "sec-fetch-site", "same-origin").status, 200)
	eq(t, request(t, app.addr, "POST", "/api/v1/auth/login", body,
		"content-type", "application/json", "origin", origin).status, 200, "Origin equal to Host")
	eq(t, app.login("ann@example.com", password).status, 200, "no browser headers at all")
}

func TestDisablingAUserSignsThemOut(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	ann := app.user("ann@example.com", false)
	cookie := app.session("ann@example.com", password)
	eq(t, app.send("PATCH", "/api/v1/users/"+ann.ID.String(), admin, obj{"status": "disabled"}).status, 200)
	eq(t, app.get("/api/v1/auth/me", cookie).status, 401)
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM sessions WHERE user_id = $1", ann.ID), int64(0))
}
