package tests

import "testing"

func TestGlobalGroupRoutesRejectOrgGroupsForNonSuperadmin(t *testing.T) {
	t.Parallel()
	w := newTopLevelWorld(t)
	app, admin, bob := w.f.app, w.f.admin, w.f.bob
	carol := app.user("carol@example.com", false)

	app.send("PUT", "/api/v1/groups/"+w.f.group+"/permissions", admin,
		obj{"permissions": obj{"groups": []string{"read", "update", "delete"}}})

	orgGroup := w.orgGroup(t, admin, w.acme, "Backend")

	for _, m := range []string{"GET", "PATCH", "DELETE"} {
		r := app.send(m, "/api/v1/groups/"+orgGroup, bob, obj{"name": "x"})
		eq(t, r.status, 404, m+" org group via global route")
	}
	eq(t, app.call("PUT", "/api/v1/groups/"+orgGroup+"/members/"+carol.ID.String(), bob).status, 404)
	eq(t, app.send("PUT", "/api/v1/groups/"+orgGroup+"/contacts", bob, obj{"contacts": []obj{}}).status, 404)

	eq(t, app.get("/api/v1/groups/"+orgGroup, admin).status, 200, "superadmin still reaches org group globally")
	eq(t, app.get(orgGroupPath(w.acme, orgGroup), admin).status, 200, "node route still works")

	owner := idOf(createGroup(t, app, admin, "Owners"))
	eq(t, app.send("PUT", ownersPath(w.acme), admin, obj{"owners": []obj{group("Owners")}}).status, 200)
	roler := idOf(createGroup(t, app, admin, "Roler"))
	app.grant(admin, w.acme, "group", "Roler", "editor")

	for _, g := range []string{owner, roler} {
		eq(t, app.send("PATCH", "/api/v1/groups/"+g, bob, obj{"name": "x"}).status, 403, "global group with bindings/ownership")
		eq(t, app.call("DELETE", "/api/v1/groups/"+g, bob).status, 403)
	}
	eq(t, app.call("DELETE", "/api/v1/groups/"+owner, admin).status, 204, "superadmin deletes it")

	harmless := idOf(createGroup(t, app, admin, "Temp"))
	eq(t, app.call("DELETE", "/api/v1/groups/"+harmless, bob).status, 204, "plain global group still deletable by groups right")
}

func TestNodeSecretListingHidesGlobalFromReaders(t *testing.T) {
	t.Parallel()
	app := startRepoSyncApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	backend := app.nodeID(root, "folder", acme, "backend")
	api := app.nodeID(root, "project", backend, "api")
	app.secret(root, "", obj{"name": "github", "ref": "env:GH_TOKEN"})
	app.secret(root, acme, obj{"name": "gitlab", "value": "glpat-org"})

	for email, role := range map[string]string{"viewer@example.com": "viewer", "editor@example.com": "editor", "auditor@example.com": "viewer"} {
		app.user(email, false)
		app.grant(root, acme, "user", email, role)
	}
	auditorGroup := idOf(createGroup(t, app, root, "Auditors"))
	eq(t, app.call("PUT", "/api/v1/groups/"+auditorGroup+"/members/"+userID(t, app, "auditor@example.com"), root).status, 204)
	app.send("PUT", "/api/v1/groups/"+auditorGroup+"/permissions", root, obj{"permissions": obj{"secrets": []string{"read"}}})

	viewer := app.session("viewer@example.com", password)
	editor := app.session("editor@example.com", password)
	auditor := app.session("auditor@example.com", password)

	eq(t, secretNames(t, app.get(secretsPath(api), viewer)), "gitlab", "viewer sees no global secret")
	eq(t, secretNames(t, app.get(secretsPath(api), editor)), "github,gitlab", "editor sees global secret")
	eq(t, secretNames(t, app.get(secretsPath(api), auditor)), "github,gitlab", "secrets:read sees global secret")
}

func secretNames(t *testing.T, r reply) string {
	t.Helper()
	eq(t, r.status, 200, r.text())
	out := ""
	for _, it := range list(t, r.json(t), "items") {
		if out != "" {
			out += ","
		}
		out += it.(obj)["name"].(string)
	}
	return out
}

func userID(t *testing.T, app *testApp, email string) string {
	t.Helper()
	return scalar[string](t, app.db, "SELECT id::text FROM users WHERE email = $1", email)
}
