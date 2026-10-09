package tests

import (
	"testing"

	"svc-registry/internal/testsupport/oidcfake"
)

func TestProviderGroupsAreKeptInSync(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t, "OAUTH_CORP_GROUP_MAP", "platform-team=>Platform;*")
	tr := newTree(app)
	bob := app.user("bob@example.com", false)
	platform := idOf(app.send("POST", "/api/v1/groups", tr.admin, obj{"name": "Platform"}).json(t))
	ops := idOf(app.send("POST", "/api/v1/groups", tr.admin, obj{"name": "Ops"}).json(t))
	eq(t, app.call("PUT", "/api/v1/groups/"+ops+"/members/"+bob.ID.String(), tr.admin).status, 204)
	app.grant(tr.admin, tr.backend, "group", "platform", "editor")

	person := oidcfake.Person{Subject: "s-bob", Email: "bob@example.com", EmailVerified: true, Groups: []string{"platform-team", "sre"}}
	idp.SignIn(person)
	bobSession := cookieNamed(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", ""), "registry_session")

	members := func(group string) map[string]string {
		out := map[string]string{}
		for _, m := range list(t, app.get("/api/v1/groups/"+group, tr.admin).json(t), "members") {
			out[m.(obj)["email"].(string)] = m.(obj)["source"].(string)
		}
		return out
	}
	eq(t, members(platform)["bob@example.com"], "oauth:corp")
	eq(t, members(ops)["bob@example.com"], "manual")
	sre := scalar[string](t, app.db, "SELECT id::text FROM groups WHERE name = 'sre'")
	eq(t, members(sre)["bob@example.com"], "oauth:corp")
	eq(t, app.createNode(bobSession, obj{"kind": "project", "parent_id": tr.backend, "slug": "p1", "name": "P"}).status, 201)

	r := app.call("DELETE", "/api/v1/groups/"+platform+"/members/"+bob.ID.String(), tr.admin)
	eq(t, r.status, 409)
	eq(t, r.json(t)["code"], any("conflict.membership_managed"))
	eq(t, members(platform)["bob@example.com"], "oauth:corp")

	person.Groups = []string{"sre"}
	idp.SignIn(person)
	bobSession = cookieNamed(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", ""), "registry_session")
	_, inPlatform := members(platform)["bob@example.com"]
	eq(t, inPlatform, false)
	eq(t, members(ops)["bob@example.com"], "manual")
	eq(t, app.createNode(bobSession, obj{"kind": "project", "parent_id": tr.backend, "slug": "p2", "name": "P"}).status, 404)

	eq(t, app.call("PUT", "/api/v1/groups/"+sre+"/members/"+bob.ID.String(), tr.admin).status, 204)
	eq(t, members(sre)["bob@example.com"], "manual")
	person.Groups = nil
	idp.SignIn(person)
	app.oauthLogin("/api/v1/auth/oauth/corp/start", "")
	eq(t, members(sre)["bob@example.com"], "manual")
	eq(t, app.call("DELETE", "/api/v1/groups/"+sre+"/members/"+bob.ID.String(), tr.admin).status, 204)
}
