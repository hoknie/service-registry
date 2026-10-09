package tests

import (
	"testing"

	"svc-registry/internal/testsupport/oidcfake"
)

func TestIdentitiesOfTheAccount(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t, "OAUTH_CORP_DISPLAY_NAME", "Corporate SSO", "OAUTH_CORP_AUTO_PROVISION", "true")
	root := app.admin()
	ann := app.user("ann@example.com", false)
	idp.SignIn(oidcfake.Person{Subject: "s-ann", Email: "ann@example.com", EmailVerified: true})
	annSession := cookieNamed(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", ""), "registry_session")

	r := app.get("/api/v1/account/identities", annSession)
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	eq(t, len(items), 1)
	item := items[0].(obj)
	eq(t, item["provider"], any("corp"))
	eq(t, item["display_name"], any("Corporate SSO"))
	eq(t, item["email"], any("ann@example.com"))
	eq(t, has(item, "created_at") && has(item, "last_login_at") && !has(item, "subject"), true)
	id := idOf(item)

	eq(t, len(list(t, app.get("/api/v1/users/"+ann.ID.String()+"/identities", root).json(t), "items")), 1)
	eq(t, app.get("/api/v1/users/"+ann.ID.String()+"/identities", annSession).status, 403)
	eq(t, app.get("/api/v1/users/00000000-0000-7000-8000-000000000000/identities", root).status, 404)
	eq(t, list(t, app.get("/api/v1/account/identities", root).json(t), "items") != nil, true)

	secret := app.pat(annSession, "read", "write")
	r = app.bearer("GET", "/api/v1/account/identities", secret, nil)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.session_required"))

	eq(t, app.call("DELETE", "/api/v1/account/identities/"+id, root).status, 404)
	eq(t, app.call("DELETE", "/api/v1/account/identities/"+id, annSession).status, 204)
	eq(t, len(list(t, app.get("/api/v1/account/identities", annSession).json(t), "items")), 0)
	eq(t, app.call("DELETE", "/api/v1/account/identities/"+id, annSession).status, 404)
}

func TestTheLastLoginMethodStays(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t, "OAUTH_CORP_AUTO_PROVISION", "true")
	idp.SignIn(oidcfake.Person{Subject: "s-new", Email: "new@example.com", EmailVerified: true})
	session := cookieNamed(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", ""), "registry_session")
	id := idOf(list(t, app.get("/api/v1/account/identities", session).json(t), "items")[0].(obj))
	r := app.call("DELETE", "/api/v1/account/identities/"+id, session)
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.last_login_method"))
	eq(t, app.send("POST", "/api/v1/auth/password", session, obj{"new_password": password}).status, 204)
	eq(t, app.call("DELETE", "/api/v1/account/identities/"+id, session).status, 204)
}
