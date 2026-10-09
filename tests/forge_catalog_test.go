package tests

import (
	"testing"

	"svc-registry/internal/testsupport/forgefake"
)

func TestManagedNodesRefuseManualMovesDeletesAndManagedFields(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	gl := app.nodeID(root, "organization", "", "gl")
	other := app.nodeID(root, "organization", "", "other")
	f := forgefake.Start(t, "gitlab", forgeToken, "acme")
	f.Put(forgefake.Repo{ID: 1, Path: "acme/platform/api"})
	conn := idOf(app.connect(root, gl, f, nil))
	app.syncNow(root, gl, conn)
	platform := idOf(app.childBySlug(root, gl, "platform"))
	api := idOf(app.childBySlug(root, platform, "api"))

	for _, r := range []reply{
		app.call("DELETE", nodePath(api), root),
		app.send("POST", nodePath(api)+"/move", root, obj{"parent_id": other}),
		app.send("POST", nodePath(platform)+"/move", root, obj{"parent_id": other}),
		app.send("PATCH", nodePath(api), root, obj{"description": "x"}),
		app.send("PATCH", nodePath(api), root, obj{"repo_url": "https://example.com/x"}),
		app.send("PATCH", nodePath(api), root, obj{"default_branch": "dev"}),
	} {
		eq(t, r.status, 409, r.text())
		eq(t, code(t, r), any("conflict.managed_by_forge"))
	}
	r := app.send("PATCH", nodePath(api), root, obj{"name": "Core API", "labels": obj{"tier": "1"}})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["name"], any("Core API"))
	eq(t, r.json(t)["managed"], any(true))
	eq(t, app.createNode(root, obj{"kind": "project", "parent_id": platform, "slug": "manual", "name": "Manual"}).status, 201)
	manual := app.childBySlug(root, platform, "manual")
	eq(t, manual["managed"], any(false))

	tree := app.get("/api/v1/catalog/tree?root="+gl+"&depth=3", root).json(t)
	found := false
	for _, n := range list(t, tree, "nodes") {
		if n.(obj)["id"] == api {
			found = true
			eq(t, n.(obj)["managed"], any(true))
		}
	}
	eq(t, found, true)
}

func TestGiteaIsAForgeOfProjects(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	org := app.nodeID(root, "organization", "", "o")
	r := app.createNode(root, obj{"kind": "project", "parent_id": org, "slug": "p", "name": "P", "forge": "gitea"})
	eq(t, r.status, 201, r.text())
	eq(t, r.json(t)["forge"], any("gitea"))
	eq(t, r.json(t)["managed"], any(false))
	got := app.get(nodePath(idOf(r.json(t))), root).json(t)
	eq(t, got["repository"], nil)
	eq(t, app.get(nodePath(idOf(r.json(t)))+"/readme", root).status, 404)
	eq(t, app.get(nodePath(org)+"/readme", root).status, 404)
}
