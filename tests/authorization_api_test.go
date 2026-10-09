package tests

import (
	"testing"
)

type catalogTree struct {
	admin, acme, globex, backend, api, secret string
}

func newTree(app *testApp) catalogTree {
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	globex := app.nodeID(admin, "organization", "", "globex")
	backend := app.nodeID(admin, "folder", acme, "backend")
	return catalogTree{
		admin: admin, acme: acme, globex: globex, backend: backend,
		api:    app.nodeID(admin, "project", backend, "api"),
		secret: app.nodeID(admin, "project", backend, "secret"),
	}
}

func bobSession(app *testApp) string {
	app.user("bob@example.com", false)
	return app.session("bob@example.com", password)
}

func TestEveryCatalogRouteNeedsASession(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	id := newID()
	for _, rt := range [][2]string{
		{"GET", "/api/v1/catalog/nodes"},
		{"POST", "/api/v1/catalog/nodes"},
		{"GET", "/api/v1/catalog/tree"},
		{"GET", nodePath(id)},
		{"PATCH", nodePath(id)},
		{"DELETE", nodePath(id)},
		{"POST", nodePath(id) + "/move"},
		{"GET", nodePath(id) + "/bindings"},
		{"POST", nodePath(id) + "/bindings"},
		{"DELETE", nodePath(id) + "/bindings/" + id},
		{"GET", nodePath(id) + "/keys"},
		{"POST", nodePath(id) + "/keys"},
		{"DELETE", nodePath(id) + "/keys/" + id},
		{"GET", nodePath(id) + "/events"},
		{"GET", nodePath(id) + "/deployments"},
		{"GET", nodePath(id) + "/environments"},
	} {
		r := app.send(rt[0], rt[1], "", obj{})
		eq(t, r.status, 401, rt[0], " ", rt[1])
		eq(t, r.json(t)["code"], any("auth.unauthenticated"))
	}
}

func TestRolesAreInheritedDownward(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	app.grant(tr.admin, tr.acme, "user", "bob@example.com", "viewer")

	r := app.get(nodePath(tr.api), bob)
	eq(t, r.status, 200)
	eq(t, r.json(t)["access"], any("read"))
	eq(t, jsonText(r.json(t)["permissions"]), `["catalog.read"]`)
	r = app.send("PATCH", nodePath(tr.api), bob, obj{"name": "X"})
	eq(t, r.status, 403)
	eq(t, r.json(t)["code"], any("auth.forbidden"))
	eq(t, app.get(nodePath(tr.globex), bob).status, 404)
	top := app.get("/api/v1/catalog/nodes", bob).json(t)
	eq(t, slugs(t, top, "items"), "acme")
	eq(t, at(top, "items", 0, "access"), any("read"))
}

func TestEditorPermissionsAndContainers(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	app.grant(tr.admin, tr.backend, "user", "bob@example.com", "editor")

	r := app.get(nodePath(tr.backend), bob).json(t)
	eq(t, jsonText(r["permissions"]), `["catalog.read","catalog.write","catalog.keys"]`)
	created := app.createNode(bob, obj{"kind": "project", "parent_id": tr.backend, "slug": "new", "name": "New"})
	eq(t, created.status, 201, created.text())
	eq(t, app.send("PATCH", nodePath(tr.backend), bob, obj{"name": "B"}).status, 200)
	eq(t, app.send("DELETE", nodePath(idOf(created.json(t))), bob, nil).status, 204)
	eq(t, app.send("DELETE", nodePath(tr.backend), bob, nil).status, 403)
	eq(t, app.send("POST", nodePath(tr.api)+"/move", bob, obj{"parent_id": tr.acme}).status, 403,
		"move needs write on the target container")
	app.grant(tr.admin, tr.acme, "user", "bob@example.com", "admin")
	r2 := app.createNode(bob, obj{"kind": "organization", "slug": "mine", "name": "Mine"})
	eq(t, r2.status, 403)
	eq(t, r2.json(t)["code"], any("auth.forbidden"))
	eq(t, app.send("POST", nodePath(tr.backend)+"/move", bob, obj{"parent_id": tr.globex}).status, 404,
		"globex is invisible to Bob")
}

func TestDeletingRequiresWriteOnTheParent(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	app.grant(tr.admin, tr.api, "user", "bob@example.com", "editor")
	eq(t, app.call("DELETE", nodePath(tr.api), bob).status, 403)
	eq(t, app.get(nodePath(tr.api), tr.admin).status, 200)
}

func TestGroupRolesApplyToMembersOnlyWhileMembers(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	bobID := app.get("/api/v1/auth/me", bob).json(t)["id"].(string)
	g := app.send("POST", "/api/v1/groups", tr.admin, obj{"name": "Platform"}).json(t)
	member := "/api/v1/groups/" + idOf(g) + "/members/" + bobID
	eq(t, app.call("PUT", member, tr.admin).status, 204)
	b := app.grant(tr.admin, tr.backend, "group", "platform", "editor")
	eq(t, b["subject_kind"], any("group"))
	eq(t, b["subject_name"], any("Platform"))

	eq(t, app.createNode(bob, obj{"kind": "project", "parent_id": tr.backend, "slug": "g1", "name": "G"}).status, 201)
	eq(t, app.call("DELETE", member, tr.admin).status, 204)
	r := app.createNode(bob, obj{"kind": "project", "parent_id": tr.backend, "slug": "g2", "name": "G"})
	eq(t, r.status, 404, "the folder is invisible again")

	eq(t, app.call("PUT", member, tr.admin).status, 204)
	eq(t, app.get(nodePath(tr.backend), bob).status, 200)
	eq(t, app.call("DELETE", "/api/v1/groups/"+idOf(g), tr.admin).status, 204)
	contains(t, app.get(nodePath(tr.backend)+"/bindings", tr.admin).text(), `"items":[]`)
	eq(t, app.get(nodePath(tr.backend), bob).status, 404)
}

func TestTheHighestRoleWins(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	app.grant(tr.admin, tr.acme, "user", "bob@example.com", "viewer")
	app.grant(tr.admin, tr.backend, "user", "bob@example.com", "admin")
	eq(t, len(list(t, app.get(nodePath(tr.api), bob).json(t), "permissions")), 4)
	eq(t, len(list(t, app.get(nodePath(tr.backend), bob).json(t), "permissions")), 4)
	eq(t, jsonText(app.get(nodePath(tr.acme), bob).json(t)["permissions"]), `["catalog.read"]`)
}

func TestSuperadminCanDoAnythingWithoutBindings(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	eq(t, app.send("POST", nodePath(tr.backend)+"/move", tr.admin, obj{"parent_id": tr.globex}).status, 200)
}

func TestAncestorsOfADeepBindingAreNavigableStubs(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	app.grant(tr.admin, tr.api, "user", "bob@example.com", "viewer")

	top := app.get("/api/v1/catalog/nodes", bob).json(t)
	eq(t, slugs(t, top, "items"), "acme")
	acme := at(top, "items", 0).(obj)
	eq(t, acme["access"], any("navigate"))
	for _, hidden := range []string{"description", "labels", "created_at", "updated_at"} {
		eq(t, has(acme, hidden), false, hidden, " leaks")
	}
	kids := app.get("/api/v1/catalog/nodes?parent="+tr.acme, bob).json(t)
	eq(t, slugs(t, kids, "items"), "backend")
	eq(t, at(kids, "items", 0, "access"), any("navigate"))
	kids = app.get("/api/v1/catalog/nodes?parent="+tr.backend, bob).json(t)
	eq(t, slugs(t, kids, "items"), "api", "siblings stay hidden")
	eq(t, at(kids, "items", 0, "access"), any("read"))
	eq(t, kids["total"], any(float64(1)))

	api := app.get(nodePath(tr.api), bob).json(t)
	eq(t, api["access"], any("read"))
	eq(t, slugs(t, api, "path"), "acme,backend")
	eq(t, at(api, "path", 0, "access"), any("navigate"))

	nav := app.get(nodePath(tr.acme), bob)
	eq(t, nav.status, 200)
	eq(t, jsonText(nav.json(t)["permissions"]), `[]`)
	eq(t, has(nav.json(t), "description"), false)

	eq(t, slugs(t, app.get("/api/v1/catalog/tree?depth=5", bob).json(t), "nodes"), "acme,backend,api")
	eq(t, app.get(nodePath(tr.secret), bob).status, 404)
	eq(t, app.get(nodePath(tr.globex), bob).status, 404)
	r := app.send("PATCH", nodePath(tr.acme), bob, obj{"name": "X"})
	eq(t, r.status, 403)
	eq(t, r.json(t)["code"], any("auth.forbidden"))
	eq(t, app.send("PATCH", nodePath(tr.secret), bob, obj{"name": "X"}).status, 404, "invisible behaves as nonexistent")
}

func TestAUserWithoutBindingsSeesNothing(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	r := app.get("/api/v1/catalog/nodes", bob)
	contains(t, r.text(), `"items":[]`)
	eq(t, r.json(t)["total"], any(float64(0)))
	contains(t, app.get("/api/v1/catalog/tree", bob).text(), `"nodes":[]`)
	eq(t, app.get(nodePath(tr.acme), bob).status, 404)
}

func TestBindingsAreManagedByScopeAdmins(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	app.user("carol@example.com", false)
	carol := app.session("carol@example.com", password)
	app.grant(tr.admin, tr.acme, "user", "carol@example.com", "admin")

	b := app.grant(carol, tr.acme, "user", "BOB@example.com", "viewer")
	eq(t, b["subject_name"], any("bob@example.com"))
	eq(t, b["inherited"], any(false))
	b2 := app.grant(carol, tr.acme, "user", "bob@example.com", "admin")
	eq(t, b2["role"], any("admin"))
	bobs := 0
	for _, i := range list(t, app.get(nodePath(tr.acme)+"/bindings", carol).json(t), "items") {
		if i.(obj)["subject_name"] == "bob@example.com" {
			bobs++
		}
	}
	eq(t, bobs, 1, "one binding per subject and node")

	first := at(app.get(nodePath(tr.backend)+"/bindings", carol).json(t), "items", 0).(obj)
	eq(t, first["inherited"], any(true))
	eq(t, first["node_id"], any(tr.acme))
	eq(t, first["node_name"], any("acme"))

	for _, c := range []struct {
		body obj
		code string
	}{
		{obj{"subject_kind": "user", "subject": "nobody@example.com", "role": "viewer"}, "validation.unknown_subject"},
		{obj{"subject_kind": "robot", "subject": "x", "role": "viewer"}, "validation.invalid_subject_kind"},
		{obj{"subject_kind": "user", "subject": "bob@example.com", "role": "owner"}, "validation.invalid_role"},
	} {
		r := app.send("POST", nodePath(tr.acme)+"/bindings", carol, c.body)
		eq(t, r.status, 400, jsonText(c.body))
		eq(t, r.json(t)["code"], any(c.code))
	}
	bid := idOf(b2)
	eq(t, app.call("DELETE", nodePath(tr.backend)+"/bindings/"+bid, carol).status, 404)
	eq(t, app.call("DELETE", nodePath(tr.acme)+"/bindings/"+bid, carol).status, 204)
	eq(t, app.get(nodePath(tr.acme), bob).status, 404)

	app.grant(tr.admin, tr.acme, "user", "bob@example.com", "editor")
	r := app.get(nodePath(tr.acme)+"/bindings", bob)
	eq(t, r.status, 403)
	eq(t, r.json(t)["code"], any("auth.forbidden"))
	eq(t, app.get(nodePath(tr.globex)+"/bindings", carol).status, 404)
}
