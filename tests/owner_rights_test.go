package tests

import (
	"testing"
)

func folderIn(parent, slug string) obj {
	return obj{"kind": "folder", "parent_id": parent, "slug": slug, "name": slug}
}

func TestCreatorOwnsTheOrganizationAndNestedNodesInherit(t *testing.T) {
	t.Parallel()
	f := newRightsFixture(t)
	f.grant(t, obj{"catalog": []string{"create"}})
	acme := f.app.nodeID(f.bob, "organization", "", "acme")
	r := f.app.get(ownersPath(acme), f.bob)
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "items", 0, "email"), any("bob@example.com"))
	eq(t, at(r.json(t), "own"), any(true))

	backend := f.app.nodeID(f.bob, "folder", acme, "backend")
	api := f.app.nodeID(f.bob, "project", backend, "api")
	r = f.app.get(ownersPath(api), f.bob)
	eq(t, at(r.json(t), "items", 0, "email"), any("bob@example.com"))
	eq(t, at(r.json(t), "items", 0, "inherited"), any(true))
	eq(t, at(r.json(t), "items", 0, "node_id"), any(acme))
	eq(t, at(r.json(t), "own"), any(false))
	eq(t, scalar[int64](t, f.app.db, "SELECT count(*) FROM node_owners"), int64(1))

	globex := f.app.nodeID(f.admin, "organization", "", "globex")
	eq(t, at(f.app.get(ownersPath(globex), f.admin).json(t), "items", 0, "email"), any("root@example.com"))
	whole := f.app.pat(f.admin, "write", "read", "catalog.root")
	r = f.app.bearer("POST", "/api/v1/catalog/nodes", whole, obj{"kind": "organization", "slug": "initech", "name": "Initech"})
	eq(t, r.status, 201, r.text())
	eq(t, at(f.app.get(ownersPath(idOf(r.json(t))), f.admin).json(t), "items", 0, "email"), any("root@example.com"))
}

func TestOwnersCreateInsideTheirSubtree(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	o.user("alice@example.com", false)
	o.user("carol@example.com", false)
	alice := o.session("alice@example.com", password)
	carol := o.session("carol@example.com", password)
	eq(t, o.setOwners(o.admin, o.acme, person("alice@example.com")).status, 200)

	r := o.createNode(alice, folderIn(o.acme, "ops"))
	eq(t, r.status, 201, r.text())
	ops := idOf(r.json(t))
	r = o.createNode(alice, obj{"kind": "project", "parent_id": ops, "slug": "deploy", "name": "Deploy"})
	eq(t, r.status, 201, r.text())
	eq(t, o.send("PATCH", nodePath(ops), alice, obj{"name": "Operations"}).status, 200)
	eq(t, jsonText(o.get(nodePath(o.acme), alice).json(t)["permissions"]), `["catalog.read","catalog.write","catalog.keys"]`)
	r = o.send("POST", nodePath(o.acme)+"/bindings", alice, obj{"subject_kind": "user", "subject": "carol@example.com", "role": "viewer"})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))

	r = o.createNode(carol, folderIn(o.acme, "intruder"))
	eq(t, r.status, 404)
	eq(t, code(t, r), any("not_found"))

	eq(t, o.setOwners(o.admin, o.backend, group("SRE")).status, 200)
	eq(t, ownerNames(t, o.get(ownersPath(o.backend), alice)), "alice,SRE")
	eq(t, o.createNode(alice, folderIn(o.backend, "tools")).status, 201)

	eq(t, o.setOwners(o.admin, o.acme).status, 200)
	eq(t, o.createNode(alice, folderIn(o.acme, "later")).status, 404)
}

func TestGroupOwnersCreateAndLoseTheRightWithTheMembership(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	bob := o.user("bob@example.com", false)
	carol := o.user("carol@example.com", false)
	bobCookie := o.session("bob@example.com", password)
	eq(t, o.call("PUT", "/api/v1/groups/"+o.platform+"/members/"+bob.ID.String(), o.admin).status, 204)
	eq(t, o.setOwners(o.admin, o.acme, group("Platform")).status, 200)
	eq(t, o.createNode(bobCookie, folderIn(o.acme, "team")).status, 201)

	g := o.get("/api/v1/groups/"+o.platform, o.admin).json(t)
	eq(t, g["members_lock"], any("bindings"))
	eq(t, o.call("PUT", "/api/v1/account/managed-groups/"+o.platform+"/members/"+carol.ID.String(), bobCookie).status, 204)

	eq(t, o.call("DELETE", "/api/v1/groups/"+o.platform+"/members/"+bob.ID.String(), o.admin).status, 204)
	r := o.createNode(bobCookie, folderIn(o.acme, "again"))
	eq(t, r.status, 404)
}

func TestOwnerRightsFollowTokenNodes(t *testing.T) {
	t.Parallel()
	o := startOwners(t)
	o.user("alice@example.com", false)
	alice := o.session("alice@example.com", password)
	globex := o.nodeID(o.admin, "organization", "", "globex")
	eq(t, o.setOwners(o.admin, o.acme, person("alice@example.com")).status, 200)
	eq(t, o.setOwners(o.admin, globex, person("alice@example.com")).status, 200)

	scoped := o.secretOn(alice, []string{globex}, "read", "write")
	r := o.bearer("POST", "/api/v1/catalog/nodes", scoped, folderIn(o.acme, "a"))
	eq(t, r.status, 404)
	eq(t, o.bearer("POST", "/api/v1/catalog/nodes", scoped, folderIn(globex, "g")).status, 201)
	whole := o.pat(alice, "read", "write")
	eq(t, o.bearer("POST", "/api/v1/catalog/nodes", whole, folderIn(o.acme, "a")).status, 201)
	readOnly := o.pat(alice, "read")
	r = o.bearer("POST", "/api/v1/catalog/nodes", readOnly, folderIn(o.acme, "b"))
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
}
