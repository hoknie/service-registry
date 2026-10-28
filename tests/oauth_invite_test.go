package tests

import (
	"net/url"
	"testing"

	"svc-registry/pkg/testing/oidcfake"
)

func TestProviderSignInAcceptsInvitations(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t)
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	invite := func(email string) string {
		r := app.send("POST", invitationsPath(acme), admin, obj{"email": email, "role": "editor", "locale": "en"})
		eq(t, r.status, 201, r.text())
		return tokenInLink.FindString(r.json(t)["link"].(string))
	}
	start := func(token string) string {
		next := url.QueryEscape("/en/invite?token=" + token)
		return "/api/v1/auth/oauth/corp/start?invite=" + token + "&next=" + next
	}

	idp.SignIn(oidcfake.Person{Subject: "s-new", Email: "New@Example.com", EmailVerified: true, Name: "New Person"})
	token := invite("new@example.com")
	r := app.oauthLogin(start(token), "")
	eq(t, r.header("Location"), "/en/invite?token="+token, "no AUTO_PROVISION, still created by the invitation")
	session := cookieNamed(t, r, "registry_session")
	eq(t, app.get("/api/v1/auth/me", session).json(t)["email"], any("new@example.com"))
	eq(t, app.get("/api/v1/invitations/"+token, "").json(t)["state"], any("accepted"))
	eq(t, app.get(nodePath(acme), session).status, 200)

	idp.SignIn(oidcfake.Person{Subject: "s-other", Email: "other@example.com", EmailVerified: true})
	token = invite("someone@example.com")
	r = app.oauthLogin(start(token), "")
	eq(t, r.header("Location"), "/login?error=auth.oauth_not_provisioned", "another address gets no account")
	eq(t, app.get("/api/v1/invitations/"+token, "").json(t)["state"], any("pending"))

	app.user("ann@example.com", false)
	idp.SignIn(ann())
	r = app.oauthLogin(start(token), "")
	eq(t, r.header("Location"), "/en/invite?token="+token+"&error=invitation.email_mismatch")
	eq(t, app.get("/api/v1/invitations/"+token, "").json(t)["state"], any("pending"))
}
