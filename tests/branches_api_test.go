package tests

import (
	"testing"
)

func seedBranches(t *testing.T, app *testApp, root string) string {
	t.Helper()
	project, key := app.projectWithKey(root, "api")
	eq(t, app.send("PATCH", nodePath(project), root, obj{"default_branch": "main"}).status, 200)
	for i, b := range []string{"release/1", "feature/a", "feature/b"} {
		r := app.ingest(project, key, deployedFrom("k-"+b, b, "", "2026-06-0"+string(rune('1'+i))+"T12:00:00Z"))
		eq(t, r.status, 201, r.text())
	}
	execSQL(t, app.db, "UPDATE branches SET last_activity_at = now() - (interval '1 hour' * length(name)) WHERE NOT is_default")
	return project
}

func TestBranchListOrderFiltersAndPages(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	project := seedBranches(t, app, root)
	eq(t, app.branchNames(root, project, ""), "main,feature/a,feature/b,release/1")
	eq(t, app.branchNames(root, project, "q=feature/"), "feature/a,feature/b")
	eq(t, app.branchNames(root, project, "q=Feature/"), "", "case-sensitive")
	eq(t, app.put(t, branchesPath(project)+"/pin?name=release%2F1", root).status, 200)
	eq(t, app.branchNames(root, project, ""), "main,release/1,feature/a,feature/b", "pinned after default")
	page := app.get(branchesPath(project)+"?limit=2&offset=1", root).json(t)
	eq(t, page["total"], any(float64(4)))
	eq(t, len(list(t, page, "items")), 2)

	execSQL(t, app.db, "UPDATE branches SET last_activity_at = now() - interval '100 days' WHERE name = 'feature/b'")
	execSQL(t, app.db, "UPDATE branches SET gone_at = now() WHERE name = 'feature/a'")
	eq(t, app.branchNames(root, project, "state=stale"), "feature/b")
	eq(t, app.branchNames(root, project, "state=gone"), "feature/a")
	eq(t, app.branchNames(root, project, "state=active"), "main,release/1")
	eq(t, app.branch(root, project, "feature/b")["stale"], any(true))

	r := app.get(branchesPath(project)+"?state=old", root)
	eq(t, code(t, r), any("validation.invalid_branch_state"))
	eq(t, code(t, app.get(branchesPath(project)+"?limit=0", root)), any("validation.invalid_pagination"))
	org := scalar[string](t, app.db, "SELECT parent_id::text FROM nodes WHERE id = $1", project)
	eq(t, app.get(branchesPath(org), root).status, 404, "not a project")
}

func TestOneBranchPinAndDelete(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	project := seedBranches(t, app, root)
	b := app.branch(root, project, "feature/a")
	eq(t, b["name"], any("feature/a"))
	eq(t, app.branch(root, project, "refs/heads/feature/a")["name"], any("feature/a"))
	eq(t, app.branch(root, project, "nope") == nil, true)
	eq(t, code(t, app.get(branchesPath(project)+"/item?name=a%20b", root)), any("validation.invalid_branch"))
	eq(t, code(t, app.get(branchesPath(project)+"/item", root)), any("validation.invalid_branch"))

	r := app.put(t, branchesPath(project)+"/pin?name=feature%2Fa", root)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["pinned"], any(true))
	eq(t, app.put(t, branchesPath(project)+"/pin?name=feature%2Fa", root).json(t)["pinned"], any(true), "idempotent")
	r = app.call("DELETE", branchesPath(project)+"/pin?name=feature%2Fa", root)
	eq(t, r.status, 200)
	eq(t, r.json(t)["pinned"], any(false))
	eq(t, app.put(t, branchesPath(project)+"/pin?name=nope", root).status, 404)

	eq(t, app.call("DELETE", branchesPath(project)+"/item?name=feature%2Fa", root).status, 204)
	eq(t, app.branch(root, project, "feature/a") == nil, true)
	eq(t, app.call("DELETE", branchesPath(project)+"/item?name=feature%2Fa", root).status, 404)
	r = app.call("DELETE", branchesPath(project)+"/item?name=main", root)
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.branch_is_default"))
	items := list(t, app.get(nodePath(project)+"/deployments?branch=feature%2Fa", root).json(t), "items")
	eq(t, len(items), 1)
}

func TestBranchRights(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	project := seedBranches(t, app, root)
	app.user("viewer@example.com", false)
	app.grant(root, project, "user", "viewer@example.com", "viewer")
	viewer := app.session("viewer@example.com", password)
	eq(t, app.get(branchesPath(project), viewer).status, 200)
	eq(t, code(t, app.put(t, branchesPath(project)+"/pin?name=main", viewer)), any("auth.forbidden"))
	eq(t, code(t, app.call("DELETE", branchesPath(project)+"/item?name=feature%2Fa", viewer)), any("auth.forbidden"))
	app.user("stranger@example.com", false)
	eq(t, app.get(branchesPath(project), app.session("stranger@example.com", password)).status, 404)
	read := app.pat(viewer, "read")
	eq(t, app.bearer("GET", branchesPath(project), read, nil).status, 200)
	eq(t, code(t, app.bearer("PUT", branchesPath(project)+"/pin?name=main", read, nil)), any("auth.insufficient_scope"))
	eq(t, app.call("POST", branchesPath(project)+"/pin?name=main", root).status, 405)
}

func (a *testApp) put(t testing.TB, path, cookie string) reply {
	t.Helper()
	return a.call("PUT", path, cookie)
}
