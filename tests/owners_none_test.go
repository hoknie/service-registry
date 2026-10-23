package tests

import "testing"

func (o *ownersTree) clearOwners(cookie, node string) reply {
	o.t.Helper()
	return o.send("PUT", ownersPath(node), cookie, obj{"owners": []obj{}, "none": true})
}

func TestOwnersCanBeExplicitlyNone(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	bob := o.user("bob@example.com", false)
	eq(t, o.call("PUT", "/api/v1/groups/"+o.platform+"/members/"+bob.ID.String(), o.admin).status, 204)
	o.grant(o.admin, o.acme, "user", "bob@example.com", "viewer")
	eq(t, o.setOwners(o.admin, o.acme, group("Platform")).status, 200)

	r := o.clearOwners(o.admin, o.backend)
	eq(t, r.status, 200, r.text())
	eq(t, r.text(), `{"items":[],"from":"`+o.backend+`","own":true,"none":true}`)
	r = o.get(ownersPath(o.api), o.admin)
	eq(t, r.text(), `{"items":[],"from":"`+o.backend+`","own":false,"none":true}`)
	eq(t, ownerNames(t, o.get(ownersPath(o.web), o.admin)), "Platform")

	session := o.session("bob@example.com", password)
	eq(t, names(t, o.get(tablePath("owner=me"), session)), "(acme),web")
	eq(t, names(t, o.get(tablePath("owner=group:"+o.platform), session)), "(acme),web")
	eq(t, jsonText(o.get("/api/v1/catalog/owners", session).json(t)["items"]),
		`[{"count":1,"name":"Platform","subject_id":"`+o.platform+`","subject_kind":"group"}]`)
	r = o.get(tablePath("parent="+o.backend), o.admin)
	eq(t, jsonText(at(r.json(t), "items", 0, "owners")), `[]`)

	r = o.send("PUT", ownersPath(o.backend), o.admin, obj{"owners": []obj{group("SRE")}, "none": true})
	eq(t, code(t, r), any("validation.invalid_owners"))
	eq(t, o.get(ownersPath(o.backend), o.admin).json(t)["none"], any(true))

	r = o.setOwners(o.admin, o.api, person("bob@example.com"))
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["none"], any(false))
	eq(t, names(t, o.get(tablePath("owner=me"), session)), "(acme),(backend),api,web")

	r = o.setOwners(o.admin, o.backend, group("SRE"))
	eq(t, ownerNames(t, r), "SRE")
	eq(t, r.json(t)["none"], any(false))
	eq(t, o.clearOwners(o.admin, o.backend).json(t)["none"], any(true))
	r = o.setOwners(o.admin, o.backend)
	eq(t, r.status, 200, r.text())
	eq(t, ownerNames(t, r), "Platform")
	eq(t, r.json(t)["from"], any(o.acme))
	eq(t, r.json(t)["none"], any(false))

	eq(t, o.clearOwners(o.admin, o.backend).status, 200)
	eq(t, o.call("DELETE", nodePath(o.api), o.admin).status, 204)
	eq(t, o.call("DELETE", nodePath(o.backend), o.admin).status, 204)
	eq(t, scalar[int64](t, o.db, "SELECT count(*) FROM node_owner_clears"), int64(0))
}

func TestVanishedLastOwnerDoesNotClearOwners(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	eq(t, o.setOwners(o.admin, o.acme, group("SRE")).status, 200)
	eq(t, o.setOwners(o.admin, o.backend, group("Platform")).status, 200)
	eq(t, o.call("DELETE", "/api/v1/groups/"+o.platform, o.admin).status, 204)
	r := o.get(ownersPath(o.api), o.admin)
	eq(t, ownerNames(t, r), "SRE")
	eq(t, r.json(t)["none"], any(false))
}
