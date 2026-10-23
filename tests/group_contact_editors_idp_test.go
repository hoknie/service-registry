package tests

import (
	"testing"

	"svc-registry/pkg/testing/oidcfake"
)

func TestProviderGrantsContactEditorRights(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t, "OAUTH_CORP_GROUP_MAP", "platform-team=>Platform;sre-team=>SRE",
		"OAUTH_CORP_CONTACT_EDITOR_MAP", "platform-leads=>Platform;sre-leads=>SRE;ops-leads=>Ops")
	tr := newTree(app)
	bob := app.user("bob@example.com", false)
	platform := idOf(app.send("POST", "/api/v1/groups", tr.admin, obj{"name": "Platform"}).json(t))
	sre := idOf(app.send("POST", "/api/v1/groups", tr.admin, obj{"name": "SRE"}).json(t))
	ops := idOf(app.send("POST", "/api/v1/groups", tr.admin, obj{"name": "Ops"}).json(t))

	editor := func(group string) any {
		for _, m := range list(t, app.get("/api/v1/groups/"+group, tr.admin).json(t), "members") {
			if m.(obj)["email"] == "bob@example.com" {
				return m.(obj)["contact_editor_source"]
			}
		}
		return "not a member"
	}
	signIn := func(groups ...string) {
		idp.SignIn(oidcfake.Person{Subject: "s-bob", Email: "bob@example.com", EmailVerified: true, Groups: groups})
		cookieNamed(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", ""), "registry_session")
	}

	signIn("platform-team", "platform-leads", "sre-team", "ops-leads")
	eq(t, editor(platform), any("oauth:corp"))
	eq(t, editor(sre), nil)
	eq(t, editor(ops), any("not a member"))
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM group_contact_editors WHERE user_id = $1", bob.ID), int64(1))

	r := app.call("DELETE", "/api/v1/groups/"+platform+"/contact-editors/"+bob.ID.String(), tr.admin)
	eq(t, r.status, 409)
	eq(t, r.json(t)["code"], any("conflict.contact_editor_managed"))
	eq(t, editor(platform), any("oauth:corp"))

	eq(t, app.call("PUT", "/api/v1/groups/"+sre+"/contact-editors/"+bob.ID.String(), tr.admin).status, 204)
	eq(t, editor(sre), any("manual"))

	signIn("platform-team", "sre-team", "sre-leads")
	eq(t, editor(platform), nil)
	eq(t, editor(sre), any("manual"))

	signIn("platform-team", "platform-leads", "sre-team")
	eq(t, editor(platform), any("oauth:corp"))
	eq(t, app.call("PUT", "/api/v1/groups/"+platform+"/contact-editors/"+bob.ID.String(), tr.admin).status, 204)
	eq(t, editor(platform), any("manual"))
	signIn("platform-team")
	eq(t, editor(platform), any("manual"))
	eq(t, editor(sre), any("not a member"))

	eq(t, app.call("DELETE", "/api/v1/groups/"+platform+"/contact-editors/"+bob.ID.String(), tr.admin).status, 204)
	eq(t, editor(platform), nil)
}
