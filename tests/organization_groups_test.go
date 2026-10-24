package tests

import (
	"testing"

	"svc-registry/pkg/testing/oidcfake"
)

func orgGroupsPath(node string) string { return nodePath(node) + "/groups" }

func orgGroupPath(node, group string) string { return orgGroupsPath(node) + "/" + group }

func (w topLevelWorld) orgGroup(t *testing.T, cookie, org, name string) string {
	t.Helper()
	r := w.f.app.send("POST", orgGroupsPath(org), cookie, obj{"name": name})
	eq(t, r.status, 201, r.text())
	return idOf(r.json(t))
}

func groupNames(t *testing.T, r reply) string {
	t.Helper()
	eq(t, r.status, 200, r.text())
	out := ""
	for _, g := range list(t, r.json(t), "items") {
		if out != "" {
			out += ","
		}
		out += g.(obj)["name"].(string)
	}
	return out
}

func TestOrganizationGroupsAreManagedByOrganizationAdmins(t *testing.T) {
	t.Parallel()
	w := newTopLevelWorld(t)
	app, bob, admin := w.f.app, w.f.bob, w.f.admin
	app.user("carol@example.com", false)
	carol := app.session("carol@example.com", password)
	app.grant(admin, w.acme, "user", "bob@example.com", "admin")
	app.grant(admin, w.acme, "user", "carol@example.com", "editor")

	r := app.send("POST", orgGroupsPath(w.acme), bob, obj{"name": "Backend"})
	eq(t, r.status, 201, r.text())
	backend := idOf(r.json(t))
	eq(t, jsonText(r.json(t)["organization"]), `{"id":"`+w.acme+`","name":"acme"}`)
	eq(t, app.get("/api/v1/groups/"+backend, admin).status, 200)
	eq(t, jsonText(app.get("/api/v1/groups/"+backend, admin).json(t)["organization"]), `{"id":"`+w.acme+`","name":"acme"}`)

	r = app.send("POST", orgGroupsPath(w.acme), carol, obj{"name": "Carol's"})
	eq(t, r.status, 403, r.text())
	eq(t, code(t, r), any("auth.forbidden"))
	list := app.get(orgGroupsPath(w.acme), carol)
	eq(t, groupNames(t, list), "Backend")
	eq(t, list.json(t)["can_manage"], any(false))
	eq(t, app.get(orgGroupsPath(w.acme), bob).json(t)["can_manage"], any(true))
	eq(t, groupNames(t, app.get(orgGroupsPath(w.api), carol)), "Backend")

	r = app.send("POST", orgGroupsPath(w.backend), admin, obj{"name": "Nested"})
	eq(t, r.status, 400, r.text())
	eq(t, code(t, r), any("validation.organization_top_level"))
	eq(t, app.send("POST", orgGroupsPath(w.globex), bob, obj{"name": "X"}).status, 404)
	eq(t, app.get(orgGroupsPath(w.globex), bob).status, 404)
	eq(t, app.get(orgGroupPath(w.globex, backend), admin).status, 404)

	r = app.send("PATCH", orgGroupPath(w.acme, backend), bob, obj{"name": "Backend team"})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["name"], any("Backend team"))
	eq(t, app.send("PATCH", orgGroupPath(w.acme, backend), carol, obj{"name": "X"}).status, 403)
	eq(t, app.call("DELETE", orgGroupPath(w.acme, backend), carol).status, 403)
	eq(t, app.call("DELETE", orgGroupPath(w.acme, backend), bob).status, 204)
	eq(t, app.get("/api/v1/groups/"+backend, admin).status, 404)

	app.user("dana@example.com", false)
	dana := app.session("dana@example.com", password)
	eq(t, app.send("POST", orgGroupsPath(w.acme), dana, obj{"name": "Dana"}).status, 404)
	w.f.grant(t, obj{"groups": []string{"create"}})
	r = app.send("POST", orgGroupsPath(w.globex), w.f.bob, obj{"name": "Globex ops"})
	eq(t, r.status, 404, r.text())
	app.grant(admin, w.globex, "user", "bob@example.com", "viewer")
	r = app.send("POST", orgGroupsPath(w.globex), w.f.bob, obj{"name": "Globex ops"})
	eq(t, r.status, 201, r.text())
}

func TestOrganizationGroupNamesAndSubjects(t *testing.T) {
	t.Parallel()
	w := newTopLevelWorld(t)
	app, admin := w.f.app, w.f.admin
	global := idOf(createGroup(t, app, admin, "Backend"))
	acmeBackend := w.orgGroup(t, admin, w.acme, "Backend")
	w.orgGroup(t, admin, w.globex, "Backend")
	r := app.send("POST", orgGroupsPath(w.acme), admin, obj{"name": "backend"})
	eq(t, r.status, 409, r.text())
	eq(t, code(t, r), any("conflict.group_name_taken"))
	eq(t, app.send("POST", "/api/v1/groups", admin, obj{"name": "BACKEND"}).status, 409)
	w.orgGroup(t, admin, w.globex, "Sre")

	b := app.grant(admin, w.api, "group", "backend", "editor")
	eq(t, b["subject_id"], any(acmeBackend))
	eq(t, app.grant(admin, w.globex, "group", "Backend", "viewer")["subject_id"] != any(acmeBackend), true)
	r = app.send("POST", nodePath(w.api)+"/bindings", admin, obj{"subject_kind": "group", "subject": "Sre", "role": "viewer"})
	eq(t, r.status, 400, r.text())
	eq(t, code(t, r), any("validation.unknown_subject"))

	owners := app.send("PUT", nodePath(w.backend)+"/owners", admin, obj{"owners": []obj{{"subject_kind": "group", "subject": "Backend"}}})
	eq(t, owners.status, 200, owners.text())
	eq(t, at(owners.json(t), "items", 0, "subject_id"), any(acmeBackend))
	r = app.send("PUT", nodePath(w.backend)+"/owners", admin, obj{"owners": []obj{{"subject_kind": "group", "subject": "Sre"}}})
	eq(t, r.status, 400, r.text())
	cands := app.get(nodePath(w.backend)+"/owners/candidates?q=back", admin)
	eq(t, cands.status, 200, cands.text())
	eq(t, len(list(t, cands.json(t), "items")), 2)
	eq(t, at(cands.json(t), "items", 0, "subject_id"), any(acmeBackend))
	eq(t, at(cands.json(t), "items", 0, "organization", "name"), any("acme"))
	eq(t, at(cands.json(t), "items", 1, "organization"), nil)

	_, err := app.db.Pool.Exec(t.Context(), "INSERT INTO role_bindings (id, node_id, group_id, role) VALUES (gen_random_uuid(), $1, $2, 'viewer')", w.globex, acmeBackend)
	eq(t, err != nil, true)

	r = app.send("PUT", "/api/v1/groups/"+acmeBackend+"/permissions", admin, obj{"permissions": obj{"users": []string{"read"}}})
	eq(t, r.status, 400, r.text())
	eq(t, code(t, r), any("validation.organization_group_rights"))
	eq(t, app.send("PUT", "/api/v1/groups/"+global+"/permissions", admin, obj{"permissions": obj{}}).status, 200)
}

func TestOrganizationGroupsBlockMovesAndVanishWithTheOrganization(t *testing.T) {
	t.Parallel()
	w := newTopLevelWorld(t)
	app, admin := w.f.app, w.f.admin
	backend := w.orgGroup(t, admin, w.acme, "Backend")
	binding := app.grant(admin, w.backend, "group", "Backend", "editor")

	r := app.send("POST", nodePath(w.acme)+"/move", admin, obj{"parent_id": w.globex})
	eq(t, r.status, 409, r.text())
	eq(t, code(t, r), any("conflict.organization_has_groups"))
	r = app.send("POST", nodePath(w.backend)+"/move", admin, obj{"parent_id": w.globex})
	eq(t, r.status, 409, r.text())
	eq(t, code(t, r), any("conflict.organization_groups_bound"))
	eq(t, app.call("DELETE", nodePath(w.backend)+"/bindings/"+idOf(binding), admin).status, 204)
	r = app.send("POST", nodePath(w.backend)+"/move", admin, obj{"parent_id": w.globex})
	eq(t, r.status, 200, r.text())

	lone := app.nodeID(admin, "organization", "", "lone")
	loneGroup := w.orgGroup(t, admin, lone, "Lone")
	eq(t, app.call("DELETE", nodePath(lone), admin).status, 204)
	eq(t, app.get("/api/v1/groups/"+loneGroup, admin).status, 404)
	eq(t, app.get("/api/v1/groups/"+backend, admin).status, 200)
}

func TestOrganizationGroupMembersAndVisibility(t *testing.T) {
	t.Parallel()
	w := newTopLevelWorld(t)
	app, bob, admin := w.f.app, w.f.bob, w.f.admin
	carol := app.user("carol@example.com", false)
	app.user("caroline@example.com", false)
	app.user("eve@example.com", false)
	carolSession := app.session("carol@example.com", password)
	eve := app.session("eve@example.com", password)
	app.grant(admin, w.acme, "user", "bob@example.com", "admin")
	app.grant(admin, w.acme, "user", "carol@example.com", "viewer")
	backend := w.orgGroup(t, bob, w.acme, "Backend")
	app.grant(admin, w.api, "group", "Backend", "editor")

	cands := func(q string) int {
		r := app.get(orgGroupPath(w.acme, backend)+"/candidates?q="+q, bob)
		eq(t, r.status, 200, r.text())
		return len(list(t, r.json(t), "items"))
	}
	eq(t, cands("ca"), 0)
	eq(t, cands("car"), 2)
	eq(t, app.call("PUT", orgGroupPath(w.acme, backend)+"/members/"+carol.ID.String(), bob).status, 204)
	eq(t, app.call("PUT", orgGroupPath(w.acme, backend)+"/members/"+w.f.bobID, bob).status, 204)
	eq(t, app.call("PUT", orgGroupPath(w.acme, backend)+"/members/"+carol.ID.String(), carolSession).status, 403)
	eq(t, app.call("PUT", orgGroupPath(w.globex, backend)+"/members/"+carol.ID.String(), admin).status, 404)
	eq(t, app.get(nodePath(w.api), carolSession).json(t)["permissions"] != nil, true)
	eq(t, jsonText(at(app.get(nodePath(w.api), carolSession).json(t), "permissions")), `["catalog.read","catalog.write","catalog.keys"]`)

	full := app.get(orgGroupPath(w.acme, backend), bob)
	eq(t, full.status, 200, full.text())
	eq(t, len(list(t, full.json(t), "members")), 2)
	eq(t, full.json(t)["can_manage_members"], any(true))
	hidden := app.get(orgGroupPath(w.acme, backend), carolSession)
	eq(t, hidden.status, 200, hidden.text())
	eq(t, len(list(t, hidden.json(t), "members")), 0)
	eq(t, hidden.json(t)["members_hidden"], any(true))
	eq(t, hidden.json(t)["member_count"], any(float64(2)))

	eq(t, app.get(orgGroupsPath(w.acme), eve).status, 404)
	w.open(t, w.acme)
	eq(t, groupNames(t, app.get(orgGroupsPath(w.acme), eve)), "Backend")
	eq(t, app.get(orgGroupPath(w.acme, backend), eve).json(t)["members_hidden"], any(true))

	own := app.get("/api/v1/account/groups", carolSession)
	eq(t, own.status, 200, own.text())
	eq(t, at(own.json(t), "items", 0, "organization", "name"), any("acme"))
}

func TestOrganizationGroupsThroughTokens(t *testing.T) {
	t.Parallel()
	w := newTopLevelWorld(t)
	app, bob, admin := w.f.app, w.f.bob, w.f.admin
	app.grant(admin, w.acme, "user", "bob@example.com", "admin")
	globexPrj := app.nodeID(admin, "project", w.globex, "billing")
	app.grant(admin, globexPrj, "user", "bob@example.com", "viewer")

	read := app.pat(bob, "read")
	eq(t, app.bearer("GET", orgGroupsPath(w.acme), read, nil).status, 200)
	eq(t, app.bearer("POST", orgGroupsPath(w.acme), read, obj{"name": "X"}).status, 403)
	write := app.pat(bob, "read", "write")
	r := app.bearer("POST", orgGroupsPath(w.acme), write, obj{"name": "Tokens"})
	eq(t, r.status, 201, r.text())
	outside := app.secretOn(bob, []string{globexPrj}, "read", "write")
	eq(t, app.bearer("GET", orgGroupsPath(w.acme), outside, nil).status, 404)
	eq(t, app.bearer("POST", orgGroupsPath(w.acme), outside, obj{"name": "Y"}).status, 404)
	inside := app.secretOn(bob, []string{w.acme}, "read", "write")
	eq(t, app.bearer("POST", orgGroupsPath(w.acme), inside, obj{"name": "Inside"}).status, 201)
}

func TestProviderSyncIgnoresOrganizationGroups(t *testing.T) {
	t.Parallel()
	app, idp := startOAuthApp(t, "OAUTH_CORP_GROUP_MAP", "*")
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	r := app.send("POST", orgGroupsPath(acme), admin, obj{"name": "Backend"})
	eq(t, r.status, 201, r.text())
	orgGroup := idOf(r.json(t))
	app.user("bob@example.com", false)
	idp.SignIn(oidcfake.Person{Subject: "s-bob", Email: "bob@example.com", EmailVerified: true, Groups: []string{"Backend"}})
	cookieNamed(t, app.oauthLogin("/api/v1/auth/oauth/corp/start", ""), "registry_session")
	eq(t, app.get("/api/v1/groups/"+orgGroup, admin).json(t)["member_count"], any(float64(0)))
	global := scalar[string](t, app.db, "SELECT id::text FROM groups WHERE name = 'Backend' AND organization_id IS NULL")
	eq(t, app.get("/api/v1/groups/"+global, admin).json(t)["member_count"], any(float64(1)))
}
