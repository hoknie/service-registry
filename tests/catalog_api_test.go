package tests

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func newID() string { return uuid.Must(uuid.NewV7()).String() }

func TestCreateNormalizesAndReturnsTheNode(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	r := app.createNode(admin, obj{"kind": "organization", "slug": " Payments ", "name": "Payments"})
	eq(t, r.status, 201, r.text())
	n := r.json(t)
	eq(t, n["slug"], any("payments"))
	eq(t, n["parent_id"], any(nil))
	eq(t, n["description"], any(""))
	contains(t, r.text(), `"labels":{}`)
	eq(t, n["access"], any("read"))
	eq(t, has(n, "forge"), false, "no repository fields on an organization")
	eq(t, has(n, "key"), false)
	eq(t, uuid.MustParse(idOf(n)).Version(), uuid.Version(7))

	labelled := app.createNode(admin, obj{"kind": "organization", "slug": "l", "name": "L",
		"labels": obj{"team": "payments", "critical": ""}})
	eq(t, labelled.status, 201)
	got := app.get(nodePath(idOf(labelled.json(t))), admin)
	eq(t, jsonText(got.json(t)["labels"]), `{"critical":"","team":"payments"}`)

	for _, c := range []struct {
		body obj
		code string
	}{
		{obj{"kind": "organization", "slug": "-bad-", "name": "X"}, "validation.invalid_slug"},
		{obj{"kind": "team", "slug": "x", "name": "X"}, "validation.invalid_kind"},
		{obj{"kind": "organization", "slug": "x", "name": " "}, "validation.invalid_node_name"},
		{obj{"kind": "organization", "slug": "x", "name": "X", "labels": []any{"a"}}, "validation.invalid_labels"},
		{obj{"kind": "organization", "slug": "x", "name": "X", "labels": obj{"a": 1}}, "validation.invalid_labels"},
		{obj{"kind": "organization", "slug": "x", "name": "X", "repo_url": "https://e.com/x"}, "validation.repo_fields_not_allowed"},
	} {
		r := app.createNode(admin, c.body)
		eq(t, r.status, 400, jsonText(c.body))
		eq(t, r.json(t)["code"], any(c.code), jsonText(c.body))
	}
}

func TestNestingRulesHold(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	eu := app.nodeID(admin, "organization", acme, "acme-eu")
	backend := app.nodeID(admin, "folder", eu, "backend")
	billing := app.nodeID(admin, "folder", backend, "billing")
	api := app.nodeID(admin, "project", billing, "api")
	app.node(admin, "project", acme, "direct")

	for _, c := range []struct{ kind, parent string }{
		{"folder", ""}, {"project", ""}, {"organization", backend}, {"folder", api}, {"project", api},
	} {
		body := obj{"kind": c.kind, "parent_id": nil, "slug": "x", "name": "X"}
		if c.parent != "" {
			body["parent_id"] = c.parent
		}
		r := app.createNode(admin, body)
		eq(t, r.status, 409, c.kind, " under ", c.parent)
		eq(t, r.json(t)["code"], any("conflict.invalid_parent"))
	}
	r := app.createNode(admin, obj{"kind": "folder", "parent_id": newID(), "slug": "x", "name": "X"})
	eq(t, r.status, 404, "unknown parent")
}

func TestSlugsAreUniqueAmongSiblingsOnly(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	globex := app.nodeID(admin, "organization", "", "globex")
	app.node(admin, "folder", acme, "backend")
	r := app.createNode(admin, obj{"kind": "project", "parent_id": acme, "slug": "Backend", "name": "B"})
	eq(t, r.status, 409)
	eq(t, r.json(t)["code"], any("conflict.slug_taken"))
	r = app.createNode(admin, obj{"kind": "organization", "slug": "acme", "name": "Again"})
	eq(t, r.json(t)["code"], any("conflict.slug_taken"), "top level is one sibling set")
	other := app.nodeID(admin, "folder", globex, "backend")

	app.node(admin, "folder", globex, "frontend")
	r = app.send("PATCH", nodePath(other), admin, obj{"slug": "frontend"})
	eq(t, r.json(t)["code"], any("conflict.slug_taken"))
	r = app.send("POST", nodePath(other)+"/move", admin, obj{"parent_id": acme})
	eq(t, r.status, 409)
	eq(t, r.json(t)["code"], any("conflict.slug_taken"))
}

func TestGetHasBreadcrumbsAndPatchChangesFields(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	backend := app.nodeID(admin, "folder", acme, "backend")
	api := app.nodeID(admin, "project", backend, "api")

	r := app.get(nodePath(api), admin)
	eq(t, r.status, 200)
	n := r.json(t)
	eq(t, slugs(t, n, "path"), "acme,backend")
	first := at(n, "path", 0).(obj)
	for _, field := range []string{"id", "kind", "slug", "name"} {
		eq(t, has(first, field), true, field)
	}
	eq(t, has(first, "description"), false)
	eq(t, len(list(t, n, "permissions")), 4)

	r = app.send("PATCH", nodePath(backend), admin, obj{"name": "Backend services", "labels": obj{"tier": "1"}})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["name"], any("Backend services"))
	eq(t, r.json(t)["slug"], any("backend"))
	eq(t, jsonText(r.json(t)["labels"]), `{"tier":"1"}`)

	for _, bad := range []string{"nope", newID()} {
		eq(t, app.get(nodePath(bad), admin).status, 404, bad)
	}
}

func TestProjectRepositoryFields(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	r := app.createNode(admin, obj{"kind": "project", "parent_id": acme, "slug": "api", "name": "API", "forge": "forgejo",
		"repo_url": "https://git.example.com/acme/api", "default_branch": "main"})
	eq(t, r.status, 201, r.text())
	p := r.json(t)
	eq(t, p["forge"], any("forgejo"))
	eq(t, p["repo_url"], any("https://git.example.com/acme/api"))
	eq(t, p["default_branch"], any("main"))
	eq(t, strings.HasPrefix(at(p, "key", "secret").(string), "svcr_"), true)

	r = app.send("PATCH", nodePath(idOf(p)), admin, obj{"default_branch": ""})
	eq(t, r.status, 200)
	eq(t, r.json(t)["default_branch"], any(nil))
	eq(t, r.json(t)["forge"], any("forgejo"), "other fields kept")

	bare := app.node(admin, "project", acme, "bare")
	eq(t, has(bare, "forge") && bare["forge"] == nil, true, "null, not absent, on a project")

	for _, c := range []struct{ field, value, code string }{
		{"forge", "svn", "validation.invalid_forge"},
		{"repo_url", "ftp://example.com/x", "validation.invalid_repo_url"},
		{"default_branch", "-x", "validation.invalid_branch"},
	} {
		body := obj{"kind": "project", "parent_id": acme, "slug": "p2", "name": "P", c.field: c.value}
		r := app.createNode(admin, body)
		eq(t, r.status, 400, c.field)
		eq(t, r.json(t)["code"], any(c.code))
	}
	r = app.send("PATCH", nodePath(acme), admin, obj{"forge": "github"})
	eq(t, r.json(t)["code"], any("validation.repo_fields_not_allowed"))
}

func TestMovesCarryTheSubtreeAndRefuseCycles(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	globex := app.nodeID(admin, "organization", "", "globex")
	backend := app.nodeID(admin, "folder", acme, "backend")
	billing := app.nodeID(admin, "folder", backend, "billing")
	api := app.nodeID(admin, "project", backend, "api")
	mv := func(node string, parent any) reply {
		return app.send("POST", nodePath(node)+"/move", admin, obj{"parent_id": parent})
	}

	r := mv(backend, billing)
	eq(t, r.status, 409)
	eq(t, r.json(t)["code"], any("conflict.cycle"))
	eq(t, mv(backend, backend).json(t)["code"], any("conflict.cycle"))
	eq(t, mv(api, api).json(t)["code"], any("conflict.cycle"))
	eq(t, mv(backend, nil).json(t)["code"], any("conflict.invalid_parent"), "folders cannot be top level")
	eq(t, mv(backend, api).json(t)["code"], any("conflict.invalid_parent"))
	eq(t, mv(backend, newID()).status, 404)
	r = app.send("POST", nodePath(backend)+"/move", admin, obj{})
	eq(t, r.json(t)["code"], any("validation.invalid_body"), "parent_id is required")

	r = mv(backend, globex)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["parent_id"], any(globex))
	p := app.get(nodePath(api), admin).json(t)
	eq(t, slugs(t, p, "path"), "globex,backend")
	sub := app.nodeID(admin, "organization", acme, "sub")
	eq(t, mv(sub, nil).status, 200)
}

func TestConcurrentMovesCannotBuildACycle(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	a := app.nodeID(admin, "folder", acme, "a")
	b := app.nodeID(admin, "folder", acme, "b")
	statuses := make(chan int, 2)
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		go func() {
			statuses <- app.send("POST", nodePath(pair[0])+"/move", admin, obj{"parent_id": pair[1]}).status
		}()
	}
	s1, s2 := <-statuses, <-statuses
	eq(t, s1+s2, 200+409, s1, " ", s2)
}

func TestDeleteOnlyEmptyNodes(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	backend := app.nodeID(admin, "folder", acme, "backend")
	api := app.nodeID(admin, "project", backend, "api")

	r := app.call("DELETE", nodePath(backend), admin)
	eq(t, r.status, 409)
	eq(t, r.json(t)["code"], any("conflict.node_not_empty"))

	app.user("bob@example.com", false)
	app.grant(admin, api, "user", "bob@example.com", "viewer")
	eq(t, app.call("DELETE", nodePath(api), admin).status, 204)
	eq(t, app.get(nodePath(api), admin).status, 404)
	for _, table := range []string{"project_keys", "role_bindings"} {
		eq(t, app.count(table), int64(0), table)
	}
	eq(t, app.call("DELETE", nodePath(backend), admin).status, 204)
}

func TestChildrenArePagedAndOrderedByKindThenName(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	app.node(admin, "project", acme, "a")
	app.node(admin, "folder", acme, "b")
	app.node(admin, "organization", acme, "c")
	app.node(admin, "folder", acme, "a-folder")

	r := app.get("/api/v1/catalog/nodes?parent="+acme, admin)
	eq(t, r.status, 200)
	eq(t, slugs(t, r.json(t), "items"), "c,a-folder,b,a")
	eq(t, r.json(t)["total"], any(float64(4)))

	page := app.get("/api/v1/catalog/nodes?parent="+acme+"&limit=2&offset=2", admin).json(t)
	eq(t, slugs(t, page, "items"), "b,a")
	eq(t, jsonText([]any{page["total"], page["limit"], page["offset"]}), "[4,2,2]")
	r = app.get("/api/v1/catalog/nodes?parent="+acme+"&offset=10", admin)
	eq(t, r.json(t)["total"], any(float64(4)), "total survives an empty page")
	contains(t, r.text(), `"items":[]`)

	eq(t, slugs(t, app.get("/api/v1/catalog/nodes", admin).json(t), "items"), "acme")
	eq(t, app.get("/api/v1/catalog/nodes?limit=0", admin).json(t)["code"], any("validation.invalid_pagination"))
	for _, bad := range []string{"nope", newID()} {
		eq(t, app.get("/api/v1/catalog/nodes?parent="+bad, admin).status, 404, bad)
	}
}

func TestTreeIsDepthLimited(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	backend := app.nodeID(admin, "folder", acme, "backend")
	app.node(admin, "project", backend, "api")

	tr := app.get("/api/v1/catalog/tree?depth=2", admin).json(t)
	eq(t, slugs(t, tr, "nodes"), "acme,backend")
	eq(t, at(tr, "nodes", 0, "depth"), any(float64(1)))
	eq(t, at(tr, "nodes", 1, "depth"), any(float64(2)))
	eq(t, at(tr, "nodes", 1, "parent_id"), any(acme))
	eq(t, tr["truncated"], any(false))

	tr = app.get("/api/v1/catalog/tree?root="+acme, admin).json(t)
	eq(t, slugs(t, tr, "nodes"), "backend,api")
	for _, bad := range []string{"0", "11", "x"} {
		r := app.get("/api/v1/catalog/tree?depth="+bad, admin)
		eq(t, r.status, 400)
		eq(t, r.json(t)["code"], any("validation.invalid_depth"))
	}
}

func TestTreeIsTruncatedAtAThousandNodes(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	acme := app.nodeID(admin, "organization", "", "acme")
	execSQL(t, app.db, "INSERT INTO nodes (id, kind, parent_id, slug, name) "+
		"SELECT gen_random_uuid(), 'folder', $1, 'f' || i, 'F' || i FROM generate_series(1, 1000) i", uuid.MustParse(acme))
	tr := app.get("/api/v1/catalog/tree?depth=2", admin).json(t)
	eq(t, len(list(t, tr, "nodes")), 1000)
	eq(t, tr["truncated"], any(true))
}

func TestProjectClusterObservation(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	org := app.nodeID(root, "organization", "", "acme")
	folder := app.node(root, "folder", org, "backend")
	eq(t, has(folder, "cluster_observation"), false, "folders have none")
	p := app.node(root, "project", org, "api")
	eq(t, p["cluster_observation"], any(true), "on by default")
	project := idOf(p)

	app.user("eve@example.com", false)
	app.user("vic@example.com", false)
	app.grant(root, project, "user", "eve@example.com", "editor")
	app.grant(root, project, "user", "vic@example.com", "viewer")
	eve := app.session("eve@example.com", password)
	vic := app.session("vic@example.com", password)
	r := app.send("PATCH", nodePath(project), eve, obj{"cluster_observation": false})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["cluster_observation"], any(false))
	eq(t, app.get(nodePath(project), vic).json(t)["cluster_observation"], any(false))
	eq(t, code(t, app.send("PATCH", nodePath(project), vic, obj{"cluster_observation": true})), any("auth.forbidden"))
	eq(t, code(t, app.send("PATCH", nodePath(project), eve, obj{"cluster_observation": "no"})), any("validation.invalid_cluster_observation"))
	eq(t, code(t, app.send("PATCH", nodePath(idOf(folder)), root, obj{"cluster_observation": false})), any("validation.repo_fields_not_allowed"))
	r = app.createNode(root, obj{"kind": "project", "parent_id": org, "slug": "web", "name": "Web", "cluster_observation": false})
	eq(t, r.status, 201, r.text())
	eq(t, r.json(t)["cluster_observation"], any(false))
}
