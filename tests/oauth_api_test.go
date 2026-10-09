package tests

import (
	"net/url"
	"strings"
	"testing"

	"svc-registry/internal/testsupport/oidcfake"
)

func TestProvidersListIsPublic(t *testing.T) {
	t.Parallel()
	app, _ := startOAuthApp(t, "OAUTH_PROVIDERS", "corp,gitlab", "OAUTH_CORP_DISPLAY_NAME", "Corporate SSO",
		"OAUTH_GITLAB_KIND", "gitlab", "OAUTH_GITLAB_CLIENT_ID", "g", "OAUTH_GITLAB_CLIENT_SECRET", "s")
	r := app.get("/api/v1/providers", "")
	eq(t, r.status, 200, r.text())
	eq(t, r.text(), `{"providers":[{"key":"corp","display_name":"Corporate SSO"},{"key":"gitlab","display_name":"gitlab"}],"password_login":"all"}`)

	plain := startApp(t)
	eq(t, plain.get("/api/v1/providers", "").text(), `{"providers":[],"password_login":"all"}`)
}

func TestStartRedirectsToTheProviderWithPKCE(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t)
	r := app.call("GET", "/api/v1/auth/oauth/corp/start?next=/catalog", "")
	eq(t, r.status, 302, r.text())
	loc, err := url.Parse(r.header("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := loc.Query()
	eq(t, loc.Scheme+"://"+loc.Host, idp.Issuer())
	eq(t, q.Get("response_type"), "code")
	eq(t, q.Get("client_id"), idp.ClientID)
	eq(t, q.Get("redirect_uri"), publicURL+"/api/v1/auth/oauth/corp/callback")
	eq(t, q.Get("code_challenge_method"), "S256")
	eq(t, q.Get("scope"), "openid profile email")
	eq(t, q.Get("state") != "" && q.Get("nonce") != "" && q.Get("code_challenge") != "", true)
	set := r.header("Set-Cookie")
	for _, attr := range []string{"registry_oauth=", "Path=/api/v1/auth/oauth", "Max-Age=600", "HttpOnly", "SameSite=Lax", "Secure"} {
		contains(t, set, attr)
	}
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM oauth_login_states"), int64(1))

	eq(t, app.call("GET", "/api/v1/auth/oauth/nope/start", "").status, 404)

	down, downIdP := startOAuthApp(t)
	downIdP.Stop()
	r = down.call("GET", "/api/v1/auth/oauth/corp/start", "")
	eq(t, r.status, 503)
	eq(t, r.json(t)["code"], any("auth.oauth_unavailable"))
}

func TestLoginLinksByVerifiedEmail(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t)
	user := app.user("ann@example.com", false)
	idp.SignIn(ann())
	r := app.oauthLogin("/api/v1/auth/oauth/corp/start?next=/catalog%3Fnode%3D1", "")
	eq(t, r.status, 302, r.text())
	eq(t, r.header("Location"), "/catalog?node=1")
	session := cookieNamed(t, r, "registry_session")
	contains(t, strings.Join(r.headers.Values("Set-Cookie"), "\n"), "registry_oauth=; Path=/api/v1/auth/oauth; Max-Age=0")
	me := app.get("/api/v1/auth/me", session).json(t)
	eq(t, me["id"], any(user.ID.String()))
	eq(t, me["login_method"], any("oauth:corp"))
	eq(t, me["has_password"], any(true))
	eq(t, scalar[string](t, app.db, "SELECT email FROM user_identities WHERE provider = 'corp' AND subject = 'sub-ann'"), "ann@example.com")
	eq(t, scalar[string](t, app.db, "SELECT method FROM sessions WHERE user_id = $1", user.ID), "oauth:corp")

	idp.SignIn(oidcfake.Person{Subject: "sub-ann", Email: "ann@new.example", EmailVerified: false})
	r = app.oauthLogin("/api/v1/auth/oauth/corp/start?next=//evil.example", "")
	eq(t, r.status, 302)
	eq(t, r.header("Location"), "/")
	eq(t, hasCookie(r, "registry_session"), true)
	eq(t, app.login("ann@example.com", password).status, 200)
	eq(t, app.get("/api/v1/auth/me", app.session("ann@example.com", password)).json(t)["login_method"], any("password"))
}

func TestCallbackStateIsOneTimeAndBoundToTheBrowser(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t)
	app.user("ann@example.com", false)
	idp.SignIn(ann())

	start := app.call("GET", "/api/v1/auth/oauth/corp/start", "")
	browser := cookieNamed(t, start, "registry_oauth")
	resp, err := client.Get(start.header("Location"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	back, _ := url.Parse(resp.Header.Get("Location"))
	r := app.call("GET", back.RequestURI(), "")
	eq(t, r.status, 302)
	eq(t, r.header("Location"), "/login?error=auth.oauth_state_invalid")
	eq(t, hasCookie(r, "registry_session"), false)
	r = app.call("GET", back.RequestURI(), browser)
	eq(t, r.header("Location"), "/login?error=auth.oauth_state_invalid")

	start = app.call("GET", "/api/v1/auth/oauth/corp/start", "")
	browser = cookieNamed(t, start, "registry_oauth")
	resp, _ = client.Get(start.header("Location"))
	resp.Body.Close()
	back, _ = url.Parse(resp.Header.Get("Location"))
	eq(t, app.call("GET", back.RequestURI(), browser).header("Location"), "/")
	r = app.call("GET", back.RequestURI(), browser)
	eq(t, r.header("Location"), "/login?error=auth.oauth_state_invalid")
	eq(t, hasCookie(r, "registry_session"), false)

	start = app.call("GET", "/api/v1/auth/oauth/corp/start", "")
	browser = cookieNamed(t, start, "registry_oauth")
	resp, _ = client.Get(start.header("Location"))
	resp.Body.Close()
	back, _ = url.Parse(resp.Header.Get("Location"))
	execSQL(t, app.db, "UPDATE oauth_login_states SET expires_at = now() - interval '1 second'")
	eq(t, app.call("GET", back.RequestURI(), browser).header("Location"), "/login?error=auth.oauth_state_invalid")
}

func TestLoginRefusals(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t, "OAUTH_CORP_AUTO_PROVISION", "true", "OAUTH_CORP_ALLOWED_DOMAINS", "example.com")
	app.user("root@example.com", true)
	app.user("off@example.com", false)
	execSQL(t, app.db, "UPDATE users SET status = 'disabled' WHERE email = 'off@example.com'")
	cases := []struct {
		person oidcfake.Person
		code   string
	}{
		{oidcfake.Person{Subject: "s1", Email: "ann@example.com", EmailVerified: false}, "auth.oauth_email_unverified"},
		{oidcfake.Person{Subject: "s2", Email: "root@example.com", EmailVerified: true}, "auth.oauth_link_required"},
		{oidcfake.Person{Subject: "s3", Email: "x@other.example", EmailVerified: true}, "auth.oauth_not_provisioned"},
		{oidcfake.Person{Subject: "s4", Email: "off@example.com", EmailVerified: true}, "auth.account_disabled"},
	}
	for _, c := range cases {
		idp.SignIn(c.person)
		r := app.oauthLogin("/api/v1/auth/oauth/corp/start", "")
		eq(t, r.header("Location"), "/login?error="+c.code, c.person.Email)
		eq(t, hasCookie(r, "registry_session"), false)
	}
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM user_identities"), int64(0))

	idp.Deny(true)
	eq(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", "").header("Location"), "/login?error=auth.oauth_denied")
	idp.Deny(false)
	idp.WrongAudience(true)
	idp.SignIn(ann())
	eq(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", "").header("Location"), "/login?error=auth.oauth_failed")
}

func TestAutoProvisionCreatesAUserWithoutPassword(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t, "OAUTH_CORP_AUTO_PROVISION", "true", "OAUTH_CORP_ALLOWED_DOMAINS", "example.com")
	idp.SignIn(oidcfake.Person{Subject: "s-new", Email: "new@example.com", EmailVerified: true, Name: "New Person"})
	r := app.oauthLogin("/api/v1/auth/oauth/corp/start", "")
	eq(t, r.header("Location"), "/")
	session := cookieNamed(t, r, "registry_session")
	me := app.get("/api/v1/auth/me", session).json(t)
	eq(t, me["email"], any("new@example.com"))
	eq(t, me["display_name"], any("New Person"))
	eq(t, me["has_password"], any(false))
	eq(t, me["is_superadmin"], any(false))

	r = app.login("new@example.com", "anything at all")
	eq(t, r.status, 401)
	eq(t, r.json(t)["code"], any("auth.invalid_credentials"))
	eq(t, app.send("POST", "/api/v1/auth/password", session, obj{"new_password": password}).status, 204)
	eq(t, app.login("new@example.com", password).status, 200)
	r = app.send("POST", "/api/v1/auth/password", session, obj{"new_password": "another long password"})
	eq(t, r.status, 400)
	eq(t, r.json(t)["code"], any("validation.invalid_body"))
}

func TestPasswordLoginModes(t *testing.T) {
	t.Parallel()
	app, _ := startOAuthApp(t, "AUTH_PASSWORD_LOGIN", "superadmins")
	app.user("ann@example.com", false)
	app.user("root@example.com", true)
	r := app.login("ann@example.com", password)
	eq(t, r.status, 401)
	eq(t, r.json(t)["code"], any("auth.invalid_credentials"))
	eq(t, app.login("root@example.com", password).status, 200)
	eq(t, app.get("/api/v1/providers", "").json(t)["password_login"], any("superadmins"))

	off, _ := startOAuthApp(t, "AUTH_PASSWORD_LOGIN", "off")
	off.user("root@example.com", true)
	eq(t, off.login("root@example.com", password).status, 401)
}

func TestLinkingAnIdentityToTheSignedInUser(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t)
	root := app.admin()
	idp.SignIn(oidcfake.Person{Subject: "s-admin", Email: "admin@corp.example", EmailVerified: true})
	eq(t, app.call("GET", "/api/v1/auth/oauth/corp/start?link=1", "").status, 401)
	r := app.oauthLogin("/api/v1/auth/oauth/corp/start?link=1&next=/catalog", root)
	eq(t, r.status, 302)
	eq(t, r.header("Location"), "/account?tab=login")
	eq(t, hasCookie(r, "registry_session"), false)
	r = app.oauthLogin("/api/v1/auth/oauth/corp/start", "")
	eq(t, r.header("Location"), "/")
	eq(t, app.get("/api/v1/auth/me", cookieNamed(t, r, "registry_session")).json(t)["email"], any("root@example.com"))

	app.user("ann@example.com", false)
	r = app.oauthLogin("/api/v1/auth/oauth/corp/start?link=1", app.session("ann@example.com", password))
	eq(t, r.header("Location"), "/account?tab=login&error=auth.oauth_identity_taken")
}
