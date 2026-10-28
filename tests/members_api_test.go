package tests

import "testing"

func membersPath(org string) string { return nodePath(org) + "/members" }

func TestOrganizationMembersAreListedAddedAndRemoved(t *testing.T) {
	t.Parallel()
	a := startApp(t)
	admin := a.admin()
	acme := a.nodeID(admin, "organization", "", "acme")
	backend := a.nodeID(admin, "folder", acme, "backend")
	api := a.nodeID(admin, "project", backend, "api")
	ann := a.user("ann@example.com", false)
	bob := a.user("bob@example.com", false)
	a.user("carol@example.com", false)
	a.grant(admin, acme, "user", "ann@example.com", "admin")
	annS := a.session("ann@example.com", password)
	a.grant(admin, acme, "user", "bob@example.com", "viewer")
	bobS := a.session("bob@example.com", password)

	r := a.get(membersPath(acme), bobS)
	eq(t, r.status, 200, r.text())
	page := r.json(t)
	eq(t, page["can_manage"], any(false))
	eq(t, fieldsOf(t, page, "source"), "role,role,owner", "the creator owns the organization")
	eq(t, at(page, "items", 0, "user", "email"), any("ann@example.com"))
	eq(t, at(page, "items", 0, "role"), any("admin"))
	eq(t, code(t, a.get(membersPath(backend), annS)), any("validation.not_an_organization"))

	eq(t, len(list(t, a.get(membersPath(acme)+"/candidates?q=ca", annS).json(t), "items")), 0)
	eq(t, at(a.get(membersPath(acme)+"/candidates?q=car", annS).json(t), "items", 0, "email"), any("carol@example.com"))
	eq(t, a.send("POST", membersPath(acme), bobS, obj{"email": "carol@example.com"}).status, 403)

	r = a.send("POST", membersPath(acme), annS, obj{"email": "Carol@Example.com", "role": "editor"})
	eq(t, r.status, 201, r.text())
	eq(t, r.json(t)["source"], any("manual"))
	eq(t, r.json(t)["role"], any("editor"))
	eq(t, at(r.json(t), "added_by", "name"), any("ann"))
	eq(t, a.send("POST", membersPath(acme), annS, obj{"email": "carol@example.com", "role": "viewer"}).status, 200)
	eq(t, a.send("POST", membersPath(acme), annS, obj{"email": "nobody@example.com"}).status, 404)
	eq(t, code(t, a.send("POST", membersPath(acme), annS, obj{"email": "x@example.com", "role": "owner"})), any("validation.invalid_member"))

	a.grant(admin, backend, "user", "bob@example.com", "editor")
	eq(t, a.send("PUT", nodePath(api)+"/owners", admin, obj{"owners": []obj{{"subject_kind": "user", "subject": "bob@example.com"}}}).status, 200)
	eq(t, a.call("DELETE", membersPath(acme)+"/"+bob.ID.String(), annS).status, 204)
	eq(t, a.get(nodePath(api), bobS).status, 404)
	left := list(t, a.get(membersPath(acme), annS).json(t), "items")
	eq(t, len(left), 3)
	eq(t, at(left[1], "user", "email"), any("carol@example.com"))

	eq(t, code(t, a.send("PATCH", membersPath(acme)+"/"+ann.ID.String(), annS, obj{"role": "viewer"})), any("conflict.last_organization_admin"))
	eq(t, code(t, a.call("DELETE", membersPath(acme)+"/"+ann.ID.String(), annS)), any("conflict.last_organization_admin"))
	r = a.send("PATCH", membersPath(acme)+"/"+ann.ID.String(), admin, obj{"role": nil})
	eq(t, code(t, r), any("conflict.last_organization_admin"))
	eq(t, a.send("PATCH", membersPath(acme)+"/"+bob.ID.String(), annS, obj{"role": "viewer"}).status, 404)

	group := idOf(createGroup(t, a, admin, "Platform"))
	eq(t, a.call("PUT", "/api/v1/groups/"+group+"/members/"+bob.ID.String(), admin).status, 204)
	a.grant(admin, backend, "group", "Platform", "viewer")
	via := a.get(membersPath(acme)+"/via-groups", annS).json(t)
	eq(t, at(via, "items", 0, "name"), any("Platform"))
	eq(t, at(via, "items", 0, "users"), any(float64(1)))
}
