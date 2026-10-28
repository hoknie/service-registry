package tests

import (
	"net/url"
	"testing"

	"github.com/google/uuid"
)

func docSpacePath(path string) string { return "/api/v1/docs/space?path=" + url.QueryEscape(path) }

type docTree struct {
	app                              *testApp
	root                             string
	rsqa, cs, route, routePrj, other string
}

func newDocTree(t *testing.T) docTree {
	t.Helper()
	app := startApp(t)
	root := app.admin()
	rsqa := app.nodeID(root, "organization", "", "rsqa")
	cs := app.nodeID(root, "folder", rsqa, "cs")
	route := app.nodeID(root, "folder", cs, "route")
	routePrj := app.nodeID(root, "project", route, "route-prj")
	other := app.nodeID(root, "project", route, "route-old")
	app.nodeID(root, "folder", cs, "empty")
	execSQL(t, app.db, "INSERT INTO knowledge_snapshots (id, project_id, branch, commit_sha, status) VALUES ($1, $2, 'main', 'abc', 'ok')",
		uuid.Must(uuid.NewV7()), routePrj)
	execSQL(t, app.db, "INSERT INTO knowledge_snapshots (id, project_id, branch, commit_sha, status) VALUES ($1, $2, 'main', 'abc', 'failed')",
		uuid.Must(uuid.NewV7()), other)
	return docTree{app: app, root: root, rsqa: rsqa, cs: cs, route: route, routePrj: routePrj, other: other}
}

func hasDocsOf(t *testing.T, children []any) map[string]any {
	t.Helper()
	out := map[string]any{}
	for _, c := range children {
		out[c.(obj)["slug"].(string)] = c.(obj)["has_docs"]
	}
	return out
}

func TestDocSpaceResolvesPathsToNodesAndDocuments(t *testing.T) {
	t.Parallel()
	tr := newDocTree(t)
	app, root := tr.app, tr.root

	r := app.get(docSpacePath("rsqa/cs/route/route-prj"), root)
	eq(t, r.status, 200, r.text())
	body := r.json(t)
	eq(t, at(body, "node", "id"), any(tr.routePrj))
	eq(t, at(body, "node", "kind"), any("project"))
	eq(t, at(body, "node", "access"), any("read"))
	var slugs []string
	for _, a := range list(t, body, "ancestors") {
		slugs = append(slugs, a.(obj)["slug"].(string))
	}
	eq(t, jsonText(slugs), `["rsqa","cs","route"]`)
	eq(t, jsonText(body["children"]), `[]`)
	eq(t, body["doc_path"], nil)
	eq(t, body["docs"] != nil, true)

	body = app.get(docSpacePath("rsqa/cs/route/route-prj/openspec/specs/Route.md"), root).json(t)
	eq(t, at(body, "node", "id"), any(tr.routePrj))
	eq(t, body["doc_path"], any("openspec/specs/Route.md"))

	body = app.get(docSpacePath("RSQA/CS/Route/Route-Prj"), root).json(t)
	eq(t, at(body, "node", "id"), any(tr.routePrj), "slugs ignore case")

	body = app.get(docSpacePath("rsqa/cs/route"), root).json(t)
	eq(t, at(body, "node", "kind"), any("folder"))
	eq(t, jsonText(hasDocsOf(t, list(t, body, "children"))), `{"route-old":false,"route-prj":true}`)
	eq(t, body["docs"], nil)

	body = app.get(docSpacePath("rsqa/cs"), root).json(t)
	eq(t, jsonText(hasDocsOf(t, list(t, body, "children"))), `{"empty":false,"route":true}`)

	body = app.get(docSpacePath(""), root).json(t)
	eq(t, body["node"], nil)
	eq(t, jsonText(hasDocsOf(t, list(t, body, "children"))), `{"rsqa":true}`)

	for _, p := range []string{"rsqa/nope", "rsqa/cs/route/nope", "nope", "rsqa//cs"} {
		r := app.get(docSpacePath(p), root)
		eq(t, r.status, 404, p)
		eq(t, code(t, r), any("not_found"), p)
	}
}

func TestDocSpaceFollowsVisibility(t *testing.T) {
	t.Parallel()
	tr := newDocTree(t)
	app, root := tr.app, tr.root
	app.user("vic@example.com", false)
	app.grant(root, tr.routePrj, "user", "vic@example.com", "viewer")
	vic := app.session("vic@example.com", password)

	body := app.get(docSpacePath(""), vic).json(t)
	eq(t, jsonText(hasDocsOf(t, list(t, body, "children"))), `{"rsqa":true}`)
	r := app.get(docSpacePath("rsqa"), vic)
	eq(t, r.status, 200, r.text())
	body = r.json(t)
	eq(t, at(body, "node", "access"), any("navigate"))
	eq(t, at(body, "node", "description"), any(""))
	eq(t, jsonText(hasDocsOf(t, list(t, body, "children"))), `{"cs":true}`)
	body = app.get(docSpacePath("rsqa/cs/route"), vic).json(t)
	eq(t, jsonText(hasDocsOf(t, list(t, body, "children"))), `{"route-prj":true}`, "invisible siblings are not listed")
	eq(t, app.get(docSpacePath("rsqa/cs/route/route-prj"), vic).status, 200)
	eq(t, app.get(docSpacePath("rsqa/cs/route/route-old"), vic).status, 404)
	eq(t, app.get(docSpacePath("rsqa/cs/empty"), vic).status, 404)

	app.user("nobody@example.com", false)
	nobody := app.session("nobody@example.com", password)
	r = app.get(docSpacePath("rsqa"), nobody)
	eq(t, r.status, 404)
	eq(t, code(t, r), any("not_found"))
	eq(t, jsonText(app.get(docSpacePath(""), nobody).json(t)["children"]), `[]`)

	eq(t, app.call("GET", docSpacePath("rsqa"), "").status, 401)
	eq(t, app.bearer("GET", docSpacePath("rsqa"), app.pat(vic, "read"), nil).status, 200)
	r = app.bearer("GET", docSpacePath("rsqa"), app.pat(vic, "write"), nil)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
}

func TestDocSpaceListsCollectedBranchesOfAProject(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "# API"})
	eq(t, d.app.collect(), 1)
	body := d.app.get(docSpacePath("acme/api"), d.admin).json(t)
	eq(t, at(body, "docs", "source"), any("repo_sync"))
	eq(t, at(body, "docs", "default_branch"), any("main"))
	branches := at(body, "docs", "branches").([]any)
	eq(t, len(branches), 1)
	eq(t, branches[0].(obj)["name"], any("main"))
	eq(t, branches[0].(obj)["collected_at"] != nil, true)
}
