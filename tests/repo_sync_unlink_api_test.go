package tests

import (
	"testing"

	"svc-registry/pkg/testing/hostingfake"
)

func importedAPI(t *testing.T) (*testApp, string, string, string, string) {
	t.Helper()
	app := startRepoSyncApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := hostingfake.Start(t, "github", hostingToken, "acme-inc")
	f.Put(hostingfake.Repo{ID: 1, Path: "acme-inc/api", Description: "The API"})
	conn := idOf(app.connect(root, acme, f, nil))
	app.syncNow(root, acme, conn)
	api := app.childBySlug(root, acme, "api")
	if api == nil {
		t.Fatal("api not imported")
	}
	return app, root, acme, conn, idOf(api)
}

func TestManagedProjectSourceIsDerivedAndReadOnly(t *testing.T) {
	t.Parallel()
	app, root, _, _, api := importedAPI(t)
	src := app.get(nodePath(api)+"/knowledge/source", root).json(t)
	eq(t, src["kind"], any("remote"))
	eq(t, src["origin"], any("repo_sync"))
	eq(t, src["hosting"], any("github"))
	eq(t, src["url"], any("https://github.example/acme-inc/api"))
	eq(t, at(src, "credentials", "mode"), any("secret"))
	r := app.send("PUT", nodePath(api)+"/knowledge/source", root, obj{"kind": "remote", "hosting": "github", "url": "https://github.com/x/y"})
	eq(t, r.status, 409, r.text())
	eq(t, code(t, r), any("conflict.managed_by_repo_sync"))
	eq(t, app.get(nodePath(api)+"/knowledge/source", root).json(t)["origin"], any("repo_sync"))
	eq(t, app.call("DELETE", nodePath(api)+"/knowledge/source", root).status, 204)
}

func TestUnlinkMakesTheProjectManualWithItsOwnSource(t *testing.T) {
	t.Parallel()
	app, root, acme, conn, api := importedAPI(t)
	r := app.call("POST", nodePath(api)+"/repo-sync/unlink", root)
	eq(t, r.status, 200, r.text())
	node := r.json(t)
	eq(t, node["id"], any(api))
	eq(t, node["managed"], any(false))
	eq(t, node["repository"], nil)
	eq(t, node["description"], any("The API"))

	src := app.get(nodePath(api)+"/knowledge/source", root).json(t)
	eq(t, src["origin"], any("own"))
	eq(t, src["kind"], any("remote"))
	eq(t, src["hosting"], any("github"))
	eq(t, src["url"], any("https://github.example/acme-inc/api"))
	eq(t, at(src, "credentials", "mode"), any("secret"))

	items := list(t, app.get(connectionPath(acme, conn)+"/exclusions", root).json(t), "items")
	eq(t, len(items), 1)
	eq(t, at(items[0], "full_path"), any("acme-inc/api"))
	eq(t, at(items[0], "project", "id"), any(api))
	eq(t, at(items[0], "project", "path"), any("acme/api"))

	run := app.syncNow(root, acme, conn)
	eq(t, run["created"], any(float64(0)), run)
	eq(t, run["updated"], any(float64(0)), run)
	eq(t, run["orphaned"], any(float64(0)), run)
	eq(t, app.get(nodePath(api), root).json(t)["managed"], any(false))

	r = app.send("PATCH", nodePath(api), root, obj{"description": "Ядро", "default_branch": "develop"})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["default_branch"], any("develop"))
	r = app.send("PUT", nodePath(api)+"/knowledge/source", root, obj{"kind": "remote", "hosting": "github", "url": "https://github.com/acme/core"})
	eq(t, r.status, 200, r.text())

	r = app.call("POST", nodePath(api)+"/repo-sync/unlink", root)
	eq(t, code(t, r), any("conflict.repo_sync_not_linked"))
}

func TestRemovingAnExclusionAdoptsTheSameProject(t *testing.T) {
	t.Parallel()
	app, root, acme, conn, api := importedAPI(t)
	eq(t, app.call("POST", nodePath(api)+"/repo-sync/unlink", root).status, 200)
	app.user("viewer@example.com", false)
	app.grant(root, acme, "user", "viewer@example.com", "viewer")
	viewer := app.session("viewer@example.com", password)
	eq(t, app.get(connectionPath(acme, conn)+"/exclusions", viewer).status, 200)
	eq(t, code(t, app.call("DELETE", connectionPath(acme, conn)+"/exclusions/1", viewer)), any("auth.forbidden"))
	eq(t, app.call("DELETE", connectionPath(acme, conn)+"/exclusions/404", root).status, 404)

	eq(t, app.call("DELETE", connectionPath(acme, conn)+"/exclusions/1", root).status, 204)
	app.runDue()
	run := app.lastRun(root, acme, conn)
	eq(t, run["created"], any(float64(0)), run)
	eq(t, run["updated"], any(float64(1)), run)
	node := app.get(nodePath(api), root).json(t)
	eq(t, node["managed"], any(true))
	eq(t, at(node, "repository", "origin"), any("repo_sync"))
	eq(t, app.get(nodePath(api)+"/knowledge/source", root).json(t)["origin"], any("repo_sync"))
	eq(t, len(list(t, app.get(connectionPath(acme, conn)+"/exclusions", root).json(t), "items")), 0)
}

func TestUnlinkNeedsWriteOnTheProject(t *testing.T) {
	t.Parallel()
	app, root, acme, _, api := importedAPI(t)
	app.user("viewer@example.com", false)
	app.grant(root, acme, "user", "viewer@example.com", "viewer")
	viewer := app.session("viewer@example.com", password)
	eq(t, code(t, app.call("POST", nodePath(api)+"/repo-sync/unlink", viewer)), any("auth.forbidden"))
	manual := app.nodeID(root, "project", acme, "manual")
	eq(t, code(t, app.call("POST", nodePath(manual)+"/repo-sync/unlink", root)), any("conflict.repo_sync_not_linked"))
	eq(t, app.call("POST", nodePath(acme)+"/repo-sync/unlink", root).status, 404)
}
