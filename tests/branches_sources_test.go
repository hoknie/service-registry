package tests

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"svc-registry/internal/testsupport/forgefake"
)

func branchesPath(project string) string { return nodePath(project) + "/branches" }

func (a *testApp) branch(cookie, project, name string) obj {
	a.t.Helper()
	r := a.get(branchesPath(project)+"/item?name="+url.QueryEscape(name), cookie)
	if r.status == 404 {
		return nil
	}
	if r.status != 200 {
		a.t.Fatalf("branch %s: %d %s", name, r.status, r.body)
	}
	return r.json(a.t)
}

func (a *testApp) branchNames(cookie, project, query string) string {
	a.t.Helper()
	r := a.get(branchesPath(project)+"?"+query, cookie)
	if r.status != 200 {
		a.t.Fatalf("branches: %d %s", r.status, r.body)
	}
	var names []string
	for _, it := range list(a.t, r.json(a.t), "items") {
		names = append(names, it.(obj)["name"].(string))
	}
	return strings.Join(names, ",")
}

func deployedFrom(key, branch, commit, at string) obj {
	e := deployed(key, "api", "production", "1.0.0", at)
	p := e["payload"].(obj)
	p["branch"] = branch
	if commit != "" {
		p["commit_sha"] = commit
	}
	return e
}

func TestEventsCreateAndRefreshBranches(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	project, key := app.projectWithKey(root, "api")
	eq(t, app.ingest(project, key, deployedFrom("k-1", "refs/heads/feature/x", "ABC1234", "2026-06-01T12:00:00Z")).status, 201)
	b := app.branch(root, project, "feature/x")
	if b == nil {
		t.Fatal("no branch from the event")
	}
	eq(t, b["head_sha"], any("abc1234"))
	eq(t, strings.Join(strs(t, b["sources"]), ","), "ingest")
	eq(t, b["is_default"], any(false))
	eq(t, b["last_activity_at"], any("2026-06-01T12:00:00Z"))

	eq(t, app.ingest(project, key, deployedFrom("k-2", "feature/x", "aaaaaaa", "2026-06-01T11:00:00Z")).status, 201)
	eq(t, app.branch(root, project, "feature/x")["head_sha"], any("abc1234"))
	eq(t, app.ingest(project, key, deployedFrom("k-3", "feature/x", "ddddddd", "2026-06-01T13:00:00Z")).status, 201)
	eq(t, app.branch(root, project, "feature/x")["head_sha"], any("ddddddd"))
	execSQL(t, app.db, "UPDATE branches SET head_sha = '1111111'")
	eq(t, app.ingest(project, key, deployedFrom("k-3", "feature/x", "ddddddd", "2026-06-01T13:00:00Z")).status, 200)
	eq(t, app.branch(root, project, "feature/x")["head_sha"], any("1111111"))
	before := app.count("branches")
	eq(t, app.ingest(project, key, deployed("k-4", "api", "production", "1.0.1", "2026-06-01T14:00:00Z")).status, 201)
	eq(t, app.count("branches"), before)
}

func TestDefaultBranchOfAProjectIsABranch(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	org := app.nodeID(root, "organization", "", "o")
	r := app.createNode(root, obj{"kind": "project", "parent_id": org, "slug": "p", "name": "P", "default_branch": "master"})
	eq(t, r.status, 201, r.text())
	project := idOf(r.json(t))
	b := app.branch(root, project, "master")
	eq(t, b["is_default"], any(true))
	eq(t, strings.Join(strs(t, b["sources"]), ","), "manual")

	eq(t, app.send("PATCH", nodePath(project), root, obj{"default_branch": "main"}).status, 200)
	eq(t, app.branch(root, project, "main")["is_default"], any(true))
	eq(t, app.branch(root, project, "master")["is_default"], any(false))
	eq(t, app.send("PATCH", nodePath(project), root, obj{"default_branch": ""}).status, 200)
	eq(t, app.branch(root, project, "main")["is_default"], any(false))
	eq(t, app.send("PATCH", nodePath(project), root, obj{"name": "Q"}).status, 200)
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM branches WHERE is_default"), int64(0), "other edits leave it")
}

func TestForgeSyncRecordsBranches(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t, "BRANCH_SYNC_MAX_PER_REPO", "3")
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "gitlab", forgeToken, "acme")
	day := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	f.Put(forgefake.Repo{ID: 1, Path: "acme/api", Branches: []forgefake.Branch{
		{Name: "main", SHA: strings.Repeat("a", 40), Protected: true, Date: day},
		{Name: "feature/a", SHA: strings.Repeat("b", 40), Date: day},
		{Name: "old", SHA: strings.Repeat("c", 40), Date: day},
	}})
	conn := idOf(app.connect(root, acme, f, nil))
	app.syncNow(root, acme, conn)
	api := idOf(app.childBySlug(root, acme, "api"))
	main := app.branch(root, api, "main")
	eq(t, main["is_default"], any(true))
	eq(t, main["protected"], any(true))
	eq(t, main["last_activity_at"], any("2026-05-01T10:00:00Z"))
	eq(t, strings.Join(strs(t, main["sources"]), ","), "forge")
	eq(t, app.branchNames(root, api, "state=all"), "main,feature/a,old")

	pushed := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	f.Put(forgefake.Repo{ID: 1, Path: "acme/api", Pushed: pushed, Branches: []forgefake.Branch{
		{Name: "main", SHA: strings.Repeat("a", 40), Protected: true, Date: day},
		{Name: "feature/a", SHA: strings.Repeat("b", 40), Date: day},
	}})
	app.syncNow(root, acme, conn)
	eq(t, app.branch(root, api, "old")["gone_at"] != nil, true)
	eq(t, app.branchNames(root, api, "state=gone"), "old")

	f.Put(forgefake.Repo{ID: 1, Path: "acme/api", Pushed: pushed.Add(time.Hour), Branches: []forgefake.Branch{
		{Name: "main", SHA: strings.Repeat("a", 40), Date: day}, {Name: "x1", SHA: strings.Repeat("1", 40), Date: day},
		{Name: "x2", SHA: strings.Repeat("2", 40), Date: day}, {Name: "x3", SHA: strings.Repeat("3", 40), Date: day},
	}})
	run := app.syncNow(root, acme, conn)
	eq(t, at(run, "problems", 0, "code"), any("branches.truncated"), run)
	eq(t, app.branch(root, api, "feature/a")["gone_at"], nil, "truncated list marks nothing gone")

	key := app.issueProjectKey(root, api)
	eq(t, app.ingest(api, key, deployedFrom("k-1", "old", "", "2026-06-02T00:00:00Z")).status, 201)
	eq(t, app.branch(root, api, "old")["gone_at"], nil)
	eq(t, strings.Join(strs(t, app.branch(root, api, "old")["sources"]), ","), "forge,ingest")
}

func TestBranchPatternsOfAConnection(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api", Branches: []forgefake.Branch{
		{Name: "main", SHA: strings.Repeat("a", 40)}, {Name: "release/1", SHA: strings.Repeat("b", 40)}, {Name: "feature/a", SHA: strings.Repeat("c", 40)},
	}})
	c := app.connect(root, acme, f, obj{"branch_include": []string{"release/*"}})
	eq(t, strings.Join(strs(t, c["branch_include"]), ","), "release/*")
	app.syncNow(root, acme, idOf(c))
	api := idOf(app.childBySlug(root, acme, "api"))
	eq(t, app.branchNames(root, api, "state=all"), "main,release/1")
	r := app.send("PATCH", connectionPath(acme, idOf(c)), root, obj{"branch_include": []string{"[bad"}})
	eq(t, code(t, r), any("validation.invalid_name_patterns"))
}

func (a *testApp) issueProjectKey(cookie, project string) string {
	a.t.Helper()
	r := a.send("POST", nodePath(project)+"/keys", cookie, obj{"grace_secs": 0})
	if r.status != 201 {
		a.t.Fatalf("key: %d %s", r.status, r.body)
	}
	return r.json(a.t)["secret"].(string)
}
