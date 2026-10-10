package tests

import (
	"testing"
	"time"

	"svc-registry/internal/testsupport/forgefake"
)

func TestFirstImportCreatesProjectsWithMetadata(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/API", Description: "The API", Topics: []string{"go", "api"}, License: "MIT",
		Stars: 7, Readme: "# API\n", Languages: map[string]int{"Go": 900, "Shell": 100}})
	f.Put(forgefake.Repo{ID: 2, Path: "acme-inc/web", Archived: true})
	f.Put(forgefake.Repo{ID: 3, Path: "acme-inc/fork", Fork: true})
	conn := idOf(app.connect(root, acme, f, nil))

	run := app.syncNow(root, acme, conn)
	eq(t, run["status"], any("succeeded"), run)
	eq(t, run["trigger"], any("manual"))
	eq(t, run["created"], any(float64(1)))

	api := app.childBySlug(root, acme, "api")
	if api == nil {
		t.Fatal("api not imported")
	}
	eq(t, api["name"], any("API"))
	eq(t, api["forge"], any("github"))
	eq(t, api["managed"], any(true))
	eq(t, api["repo_url"], any("https://github.example/acme-inc/API"))
	eq(t, api["default_branch"], any("main"))
	eq(t, api["description"], any("The API"))
	eq(t, app.childBySlug(root, acme, "web") == nil, true, "archived skipped")
	eq(t, app.childBySlug(root, acme, "fork") == nil, true, "fork skipped")

	got := app.get(nodePath(idOf(api)), root).json(t)
	repo := got["repository"].(obj)
	eq(t, at(repo, "topics", 1), any("api"))
	eq(t, repo["license"], any("MIT"))
	eq(t, repo["stars"], any(float64(7)))
	eq(t, at(repo, "languages", "Go"), any(float64(90)))
	eq(t, repo["has_readme"], any(true))
	eq(t, repo["orphaned_at"], nil)
	eq(t, repo["visibility"], any("public"))
	readme := app.get(nodePath(idOf(api))+"/readme", root)
	eq(t, readme.status, 200)
	eq(t, readme.json(t)["markdown"], any("# API\n"))
	eq(t, readme.json(t)["kind"], any("github"))

	r := app.send("PATCH", connectionPath(acme, conn), root, obj{"include_archived": true, "include_forks": true})
	eq(t, r.status, 200, r.text())
	run = app.syncNow(root, acme, conn)
	eq(t, run["created"], any(float64(2)))
	eq(t, run["updated"], any(float64(1)))
}

func TestFiltersByName(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "gitea", forgeToken, "acme")
	for i, name := range []string{"svc-api", "svc-old", "tools"} {
		f.Put(forgefake.Repo{ID: int64(i + 1), Path: "acme/" + name})
	}
	conn := idOf(app.connect(root, acme, f, obj{"name_include": []string{"SVC-*"}, "name_exclude": []string{"*-old"}}))
	app.syncNow(root, acme, conn)
	eq(t, app.childBySlug(root, acme, "svc-api") != nil, true)
	eq(t, app.childBySlug(root, acme, "svc-old") == nil, true)
	eq(t, app.childBySlug(root, acme, "tools") == nil, true)
	eq(t, app.childBySlug(root, acme, "svc-api")["forge"], any("gitea"))
}

func TestRenameAndMoveOnTheForgeKeepTheProject(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	gl := app.nodeID(root, "organization", "", "gl")
	f := forgefake.Start(t, "gitlab", forgeToken, "acme")
	f.Put(forgefake.Repo{ID: 10, Path: "acme/platform/billing/api", Name: "api"})
	conn := idOf(app.connect(root, gl, f, nil))
	app.syncNow(root, gl, conn)

	platform := app.childBySlug(root, gl, "platform")
	if platform == nil {
		t.Fatal("subgroup folder missing")
	}
	eq(t, platform["kind"], any("folder"))
	eq(t, platform["managed"], any(true))
	billing := app.childBySlug(root, idOf(platform), "billing")
	api := app.childBySlug(root, idOf(billing), "api")
	if api == nil {
		t.Fatal("project missing")
	}
	projectID := idOf(api)
	key := app.get(projectID2keys(projectID), root).json(t)
	keyID := at(key, "items", 0, "id")

	f.Put(forgefake.Repo{ID: 10, Path: "acme/platform/core-api", Name: "core-api", Pushed: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)})
	run := app.syncNow(root, gl, conn)
	eq(t, run["status"], any("succeeded"), run)
	moved := app.childBySlug(root, idOf(platform), "core-api")
	if moved == nil {
		t.Fatal("not moved/renamed")
	}
	eq(t, idOf(moved), projectID, "same project")
	eq(t, at(app.get(projectID2keys(projectID), root).json(t), "items", 0, "id"), keyID, "keys kept")
	eq(t, app.childBySlug(root, idOf(billing), "api") == nil, true)
}

func projectID2keys(id string) string { return nodePath(id) + "/keys" }

func TestSlugConflictWithAManualNodeSkipsTheRepository(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	manual := app.node(root, "project", acme, "api")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api", Description: "forge"})
	conn := idOf(app.connect(root, acme, f, nil))
	run := app.syncNow(root, acme, conn)
	eq(t, run["status"], any("succeeded"))
	eq(t, run["skipped"], any(float64(1)))
	eq(t, at(run, "problems", 0, "code"), any("conflict.slug_taken"))
	eq(t, at(run, "problems", 0, "full_path"), any("acme-inc/api"))
	after := app.get(nodePath(idOf(manual)), root).json(t)
	eq(t, after["description"], any(""), "manual node untouched")
	eq(t, after["managed"], any(false))
	eq(t, after["repository"], nil)
}

func TestVanishedRepositoriesAreOrphanedAndReturn(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api"})
	conn := idOf(app.connect(root, acme, f, nil))
	app.syncNow(root, acme, conn)
	api := idOf(app.childBySlug(root, acme, "api"))

	f.Delete(1)
	run := app.syncNow(root, acme, conn)
	eq(t, run["orphaned"], any(float64(1)))
	got := app.get(nodePath(api), root).json(t)
	eq(t, got["managed"], any(false))
	eq(t, at(got, "repository", "orphaned_at") != nil, true)

	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api"})
	app.syncNow(root, acme, conn)
	got = app.get(nodePath(api), root).json(t)
	eq(t, got["managed"], any(true))
	eq(t, at(got, "repository", "orphaned_at"), nil)

	f.Delete(1)
	app.syncNow(root, acme, conn)
	eq(t, app.call("DELETE", nodePath(api), root).status, 204, "an orphan can be deleted")
}

func TestDetailsAreFetchedOnlyForChanges(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api", Readme: "v1"})
	f.Put(forgefake.Repo{ID: 2, Path: "acme-inc/web"})
	conn := idOf(app.connect(root, acme, f, nil))
	app.syncNow(root, acme, conn)
	eq(t, f.Calls("readme"), 2)
	app.syncNow(root, acme, conn)
	eq(t, f.Calls("readme"), 2, "nothing changed")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api", Readme: "v2", Pushed: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)})
	app.syncNow(root, acme, conn)
	eq(t, f.Calls("readme"), 3, "only the pushed one")
	api := idOf(app.childBySlug(root, acme, "api"))
	eq(t, app.get(nodePath(api)+"/readme", root).json(t)["markdown"], any("v2"))
}

func TestRateLimitPausesTheConnection(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api"})
	conn := idOf(app.connect(root, acme, f, nil))
	reset := time.Now().Add(10 * time.Minute).Truncate(time.Second)
	f.RateLimitUntil(reset)
	run := app.syncNow(root, acme, conn)
	eq(t, run["status"], any("rate_limited"))
	eq(t, run["error_code"], any("forge.rate_limited"))
	next := scalar[time.Time](t, app.db, "SELECT next_run_at FROM forge_connections")
	eq(t, next.Unix(), reset.Unix(), "next run at the reset")
	eq(t, app.runDue(), 0, "not due before the reset")
}

func TestUnusableCredentialsFailTheRun(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "gitlab", forgeToken, "acme")
	conn := idOf(app.connect(root, acme, f, obj{"credentials": app.tokenOn(root, acme, "wrong")}))
	run := app.syncNow(root, acme, conn)
	eq(t, run["status"], any("failed"))
	eq(t, run["error_code"], any("forge.unauthorized"))

	otherOrg := app.nodeID(root, "organization", "", "other")
	ref := idOf(app.connect(root, otherOrg, forgefake.Start(t, "gitea", forgeToken, "x"),
		obj{"credentials": app.refOn(root, otherOrg, "env:SVCR_TEST_UNSET_FORGE_TOKEN")}))
	other := scalar[string](t, app.db, "SELECT node_id::text FROM forge_connections WHERE id = $1", ref)
	run = app.syncNow(root, other, ref)
	eq(t, run["error_code"], any("forge.credentials_unavailable"))
}

func TestTokenFromAFileReference(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api"})
	file := writeTemp(t, forgeToken+"\n")
	conn := idOf(app.connect(root, acme, f, obj{"credentials": app.refOn(root, acme, "file:"+file)}))
	eq(t, app.syncNow(root, acme, conn)["status"], any("succeeded"))
	eq(t, app.childBySlug(root, acme, "api") != nil, true)
}
