package tests

import (
	"testing"

	"svc-registry/internal/testsupport/oidcfake"
)

func TestServiceUsersCannotSignIn(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t, "OAUTH_CORP_AUTO_PROVISION", "true", "OAUTH_CORP_ALLOWED_DOMAINS", "example.com")
	admin := app.admin()

	r := app.send("POST", "/api/v1/users", admin, obj{"email": "ci@example.com", "display_name": "CI", "is_service": true})
	eq(t, r.status, 201, r.text())
	eq(t, r.json(t)["is_service"], any(true))
	eq(t, r.json(t)["has_password"], any(false))
	ci := idOf(r.json(t))
	r = app.send("POST", "/api/v1/users", admin, obj{"email": "x@example.com", "display_name": "X"})
	eq(t, code(t, r), any("validation.invalid_body"), "a person needs a password")

	idp.SignIn(oidcfake.Person{Subject: "s-ci", Email: "ci@example.com", EmailVerified: true})
	eq(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", "").header("Location"), "/login?error=auth.service_account")
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM user_identities"), int64(0))

	bob := app.user("bob@example.com", false)
	bobSession := app.session("bob@example.com", password)
	token := app.pat(bobSession, "read")
	r = app.send("PATCH", "/api/v1/users/"+bob.ID.String(), admin, obj{"is_service": true})
	eq(t, r.status, 200, r.text())
	eq(t, app.get("/api/v1/auth/me", bobSession).status, 401, "sessions are closed")
	eq(t, app.bearer("GET", "/api/v1/auth/me", token, nil).status, 200, "tokens keep working")
	eq(t, app.login("bob@example.com", password).status, 401)
	eq(t, code(t, app.login("bob@example.com", password)), any("auth.invalid_credentials"))

	me := app.get("/api/v1/auth/me", admin).json(t)["id"].(string)
	r = app.send("PATCH", "/api/v1/users/"+me, admin, obj{"is_service": true})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.self_service"))
	_ = ci
}

func TestSuperadminIssuesTokensForUsers(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	r := app.send("POST", "/api/v1/users", admin, obj{"email": "ci@example.com", "display_name": "CI", "is_service": true})
	ci := idOf(r.json(t))
	body := obj{"name": "ci", "scopes": []string{"read", "write"}, "expires_in_days": 90}

	r = app.send("POST", "/api/v1/users/"+ci+"/tokens", admin, body)
	eq(t, r.status, 201, r.text())
	secret := r.json(t)["secret"].(string)
	me := app.bearer("GET", "/api/v1/auth/me", secret, nil)
	eq(t, me.status, 200, me.text())
	eq(t, me.json(t)["email"], any("ci@example.com"))
	eq(t, len(list(t, app.get("/api/v1/users/"+ci+"/tokens", admin).json(t), "items")), 1)

	app.user("bob@example.com", false)
	bob := app.session("bob@example.com", password)
	eq(t, app.send("POST", "/api/v1/users/"+ci+"/tokens", bob, body).status, 403)
	adminToken := app.pat(admin, "read", "write", "admin")
	r = app.bearer("POST", "/api/v1/users/"+ci+"/tokens", adminToken, body)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.session_required"))

	eq(t, app.send("PATCH", "/api/v1/users/"+ci, admin, obj{"status": "disabled"}).status, 200)
	r = app.send("POST", "/api/v1/users/"+ci+"/tokens", admin, body)
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.user_disabled"))
	eq(t, app.send("POST", "/api/v1/users/00000000-0000-7000-8000-000000000000/tokens", admin, body).status, 404)
}

func TestGroupsOfAUser(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	bob := app.user("bob@example.com", false)
	r := app.send("POST", "/api/v1/groups", admin, obj{"name": "Platform"})
	eq(t, r.status, 201, r.text())
	group := idOf(r.json(t))
	app.send("POST", "/api/v1/groups", admin, obj{"name": "Other"})
	eq(t, app.send("PUT", "/api/v1/groups/"+group+"/members/"+bob.ID.String(), admin, nil).status, 204)
	r = app.get("/api/v1/users/"+bob.ID.String()+"/groups", admin)
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	eq(t, len(items), 1)
	eq(t, at(items[0], "name"), any("Platform"))
	app.user("ann@example.com", false)
	eq(t, app.get("/api/v1/users/"+bob.ID.String()+"/groups", app.session("ann@example.com", password)).status, 403)
	eq(t, app.get("/api/v1/users/00000000-0000-7000-8000-000000000000/groups", admin).status, 404)
}
