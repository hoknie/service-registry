package tests

import "testing"

func TestOwnersNoneIsRefused(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	eq(t, o.setOwners(o.admin, o.acme, group("Platform")).status, 200)
	for _, body := range []obj{{"owners": []obj{}, "none": true}, {"owners": []obj{group("SRE")}, "none": true}} {
		r := o.send("PUT", ownersPath(o.backend), o.admin, body)
		eq(t, r.status, 400, r.text())
		eq(t, code(t, r), any("validation.invalid_owners"))
	}
	eq(t, ownerNames(t, o.get(ownersPath(o.backend), o.admin)), "Platform")
	r := o.send("PUT", ownersPath(o.backend), o.admin, obj{"owners": []obj{group("SRE")}, "none": false})
	eq(t, r.status, 200, r.text())
	eq(t, ownerNames(t, r), "Platform,SRE")
}

func TestInheritedOwnersCannotBeRemovedOrRestated(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	o.user("bob@example.com", false)
	eq(t, o.setOwners(o.admin, o.acme, group("Platform")).status, 200)

	r := o.setOwners(o.admin, o.api, group("platform"), person("bob@example.com"))
	eq(t, r.status, 200, r.text())
	eq(t, ownerNames(t, r), "Platform,bob")
	eq(t, at(r.json(t), "items", 0, "inherited"), any(true))
	eq(t, at(r.json(t), "items", 0, "node_id"), any(o.acme))
	eq(t, at(r.json(t), "items", 1, "inherited"), any(false))
	eq(t, scalar[int64](t, o.db, "SELECT count(*) FROM node_owners WHERE node_id = $1", o.api), int64(1))

	r = o.setOwners(o.admin, o.api)
	eq(t, ownerNames(t, r), "Platform")
	eq(t, r.json(t)["own"], any(false))

	eq(t, o.setOwners(o.admin, o.backend, group("Frontend")).status, 200)
	eq(t, o.setOwners(o.admin, o.acme, group("Platform"), group("Frontend")).status, 200)
	r = o.get(ownersPath(o.api), o.admin)
	eq(t, ownerNames(t, r), "Platform,Frontend")
	eq(t, at(r.json(t), "items", 1, "node_id"), any(o.acme))
	eq(t, o.setOwners(o.admin, o.acme, group("Platform")).status, 200)
	r = o.get(ownersPath(o.api), o.admin)
	eq(t, ownerNames(t, r), "Platform,Frontend")
	eq(t, at(r.json(t), "items", 1, "node_id"), any(o.backend))
}

func TestFiltersAndSuggestionsCountInheritedOwners(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	bob := o.user("bob@example.com", false)
	eq(t, o.call("PUT", "/api/v1/groups/"+o.platform+"/members/"+bob.ID.String(), o.admin).status, 204)
	o.grant(o.admin, o.acme, "user", "bob@example.com", "viewer")
	eq(t, o.setOwners(o.admin, o.acme, group("Platform")).status, 200)
	eq(t, o.setOwners(o.admin, o.api, group("Frontend")).status, 200)
	session := o.session("bob@example.com", password)

	eq(t, names(t, o.get(tablePath("owner=group:"+o.platform), session)), "(acme),(backend),api,web")
	eq(t, names(t, o.get(tablePath("owner=group:"+o.frontend), session)), "(acme),(backend),api")
	eq(t, names(t, o.get(tablePath("owner=me"), session)), "(acme),(backend),api,web")
	r := o.get(tablePath("parent="+o.backend), session)
	eq(t, jsonText(at(r.json(t), "items", 0, "owners")),
		`[{"name":"Platform","subject_id":"`+o.platform+`","subject_kind":"group"},{"name":"Frontend","subject_id":"`+o.frontend+`","subject_kind":"group"}]`)
	eq(t, jsonText(o.get("/api/v1/catalog/owners", session).json(t)["items"]),
		`[{"count":2,"name":"Platform","subject_id":"`+o.platform+`","subject_kind":"group"},`+
			`{"count":1,"name":"Frontend","subject_id":"`+o.frontend+`","subject_kind":"group"}]`)
}

func TestVanishedOwnerLeavesTheRestOfTheChain(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	eq(t, o.setOwners(o.admin, o.acme, group("SRE")).status, 200)
	eq(t, o.setOwners(o.admin, o.backend, group("Platform")).status, 200)
	eq(t, o.call("DELETE", "/api/v1/groups/"+o.platform, o.admin).status, 204)
	r := o.get(ownersPath(o.api), o.admin)
	eq(t, ownerNames(t, r), "SRE")
	eq(t, r.json(t)["own"], any(false))
}

func TestLowerOwnersAddRightsBesideUpperOnes(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	o.user("alice@example.com", false)
	dan := o.user("dan@example.com", false)
	eq(t, o.call("PUT", "/api/v1/groups/"+o.sre+"/members/"+dan.ID.String(), o.admin).status, 204)
	eq(t, o.setOwners(o.admin, o.acme, person("alice@example.com")).status, 200)
	eq(t, o.setOwners(o.admin, o.backend, group("SRE")).status, 200)
	alice, danCookie := o.session("alice@example.com", password), o.session("dan@example.com", password)

	r := o.get(ownersPath(o.backend), danCookie)
	eq(t, ownerNames(t, r), "alice,SRE")
	eq(t, at(r.json(t), "items", 0, "inherited"), any(true))
	eq(t, o.createNode(alice, folderIn(o.backend, "tools")).status, 201)
	eq(t, o.createNode(danCookie, folderIn(o.backend, "ops")).status, 201)
	r = o.createNode(danCookie, folderIn(o.acme, "outside"))
	eq(t, r.status, 403, r.text())
	eq(t, code(t, r), any("auth.forbidden"))

	scoped := o.secretOn(danCookie, []string{o.backend}, "read", "write")
	eq(t, o.bearer("POST", "/api/v1/catalog/nodes", scoped, folderIn(o.backend, "bot")).status, 201)
	eq(t, o.bearer("POST", "/api/v1/catalog/nodes", scoped, folderIn(o.acme, "bot")).status, 403)
	aliceScoped := o.secretOn(alice, []string{o.web}, "read", "write")
	eq(t, o.bearer("POST", "/api/v1/catalog/nodes", aliceScoped, folderIn(o.backend, "elsewhere")).status, 404)
}
