package tests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"svc-registry/pkg/testing/forgefake"
)

func (a *testApp) patOn(cookie string, nodes []string, scopes ...string) reply {
	a.t.Helper()
	return a.issue(cookie, obj{"name": "scoped", "scopes": scopes, "expires_in_days": 30, "nodes": nodes})
}

func (a *testApp) secretOn(cookie string, nodes []string, scopes ...string) string {
	a.t.Helper()
	r := a.patOn(cookie, nodes, scopes...)
	if r.status != 201 {
		a.t.Fatalf("issue: %d %s", r.status, r.body)
	}
	return r.json(a.t)["secret"].(string)
}

func TestTokenNodesAreCheckedAndDescribed(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	app.grant(tr.admin, tr.acme, "user", "bob@example.com", "editor")

	r := app.patOn(bob, []string{tr.backend}, "read", "write")
	eq(t, r.status, 201, r.text())
	eq(t, jsonText(r.json(t)["nodes"]), fmt.Sprintf(`[{"id":%q,"kind":"folder","name":"backend","path":"acme/backend"}]`, tr.backend))
	whole := app.issue(bob, obj{"name": "all", "scopes": []string{"read"}, "expires_in_days": 30})
	eq(t, whole.json(t)["nodes"], nil)
	contains(t, app.issue(bob, obj{"name": "null", "scopes": []string{"read"}, "expires_in_days": 30, "nodes": nil}).text(), `"nodes":null`)
	listed := app.get(tokensPath, bob)
	eq(t, jsonText(tokenByID(t, listed, idOf(r.json(t)))["nodes"]), jsonText(r.json(t)["nodes"]))

	many := make([]string, 21)
	for i := range many {
		many[i] = uuid.NewString()
	}
	for name, nodes := range map[string][]string{
		"unreadable": {tr.globex}, "random": {uuid.NewString()}, "empty": {}, "repeated": {tr.api, tr.api},
		"too many": many, "not an id": {"acme"},
	} {
		r := app.patOn(bob, nodes, "read")
		eq(t, r.status, 400, name)
		eq(t, code(t, r), any("validation.invalid_token_nodes"), name)
	}
	r = app.patOn(tr.admin, []string{tr.acme}, "read", "users:read")
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_token_nodes"))
	eq(t, app.count("personal_access_tokens"), int64(3))

	app.user("carol@example.com", false)
	carol := app.session("carol@example.com", password)
	app.grant(tr.admin, tr.api, "user", "carol@example.com", "viewer")
	r = app.patOn(carol, []string{tr.backend}, "read")
	eq(t, code(t, r), any("validation.invalid_token_nodes"), "a navigation stub is not readable")

	bot := app.user("ci-bot@example.com", false)
	app.grant(tr.admin, tr.api, "user", "ci-bot@example.com", "editor")
	path := "/api/v1/users/" + bot.ID.String() + "/tokens"
	r = app.send("POST", path, tr.admin, obj{"name": "ci", "scopes": []string{"read", "write"}, "expires_in_days": 30, "nodes": []string{tr.api}})
	eq(t, r.status, 201, r.text())
	eq(t, at(r.json(t), "nodes", 0, "path"), any("acme/backend/api"))
	r = app.send("POST", path, tr.admin, obj{"name": "ci", "scopes": []string{"read"}, "expires_in_days": 30, "nodes": []string{tr.globex}})
	eq(t, code(t, r), any("validation.invalid_token_nodes"), "nodes are checked against the owner, not the issuer")
	eq(t, at(tokenByID(t, app.get(path, tr.admin), idOf(app.get(path, tr.admin).json(t)["items"].([]any)[0].(obj))), "nodes", 0, "kind"), any("project"))
	contains(t, app.get("/api/v1/tokens", tr.admin).text(), `"path":"acme/backend/api"`)
}

func TestTokenNodesNarrowTheOwnersRoles(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	frontend := app.nodeID(tr.admin, "folder", tr.acme, "frontend")
	bob := bobSession(app)
	app.grant(tr.admin, tr.acme, "user", "bob@example.com", "editor")
	secret := app.secretOn(bob, []string{tr.backend}, "read", "write")

	eq(t, app.bearer("PATCH", nodePath(tr.api), secret, obj{"name": "API"}).status, 200)
	eq(t, app.bearer("GET", nodePath(frontend), secret, nil).status, 404)
	eq(t, app.bearer("GET", nodePath(tr.globex), secret, nil).status, 404)
	acme := app.bearer("GET", nodePath(tr.acme), secret, nil)
	eq(t, acme.status, 200)
	eq(t, acme.json(t)["access"], any("navigate"))
	eq(t, jsonText(acme.json(t)["permissions"]), `[]`)
	r := app.bearer("PATCH", nodePath(tr.acme), secret, obj{"name": "X"})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	eq(t, app.bearer("GET", nodePath(tr.backend), secret, nil).json(t)["access"], any("read"))

	top := app.bearer("GET", "/api/v1/catalog/nodes", secret, nil).json(t)
	eq(t, slugs(t, top, "items"), "acme")
	eq(t, slugs(t, app.bearer("GET", "/api/v1/catalog/nodes?parent="+tr.acme, secret, nil).json(t), "items"), "backend")
	eq(t, slugs(t, app.bearer("GET", "/api/v1/catalog/tree?depth=5", secret, nil).json(t), "nodes"), "acme,backend,api,secret")
	table := app.bearer("GET", "/api/v1/catalog/table?q=end", secret, nil).text()
	contains(t, table, `"slug":"backend"`)
	lacks(t, table, `"slug":"frontend"`)
	eq(t, app.get(nodePath(frontend), bob).status, 200, "the session is not narrowed")

	r = app.bearer("POST", "/api/v1/catalog/nodes", secret, obj{"kind": "folder", "parent_id": tr.backend, "slug": "jobs", "name": "Jobs"})
	eq(t, r.status, 201, r.text())
	r = app.bearer("POST", "/api/v1/catalog/nodes", secret, obj{"kind": "folder", "parent_id": tr.acme, "slug": "ops", "name": "Ops"})
	eq(t, r.status, 403, r.text())
}

func TestTokenNodesFollowRoleChanges(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	b := app.grant(tr.admin, tr.backend, "user", "bob@example.com", "viewer")
	secret := app.secretOn(bob, []string{tr.backend}, "read")
	eq(t, app.call("DELETE", nodePath(tr.backend)+"/bindings/"+idOf(b), tr.admin).status, 204)
	app.grant(tr.admin, tr.api, "user", "bob@example.com", "viewer")

	eq(t, app.bearer("GET", nodePath(tr.api), secret, nil).json(t)["access"], any("read"))
	eq(t, app.bearer("GET", nodePath(tr.backend), secret, nil).json(t)["access"], any("navigate"))
	eq(t, app.bearer("GET", nodePath(tr.secret), secret, nil).status, 404)
}

func TestTokenNodesSurviveMovesAndDeletes(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	bob := bobSession(app)
	app.grant(tr.admin, tr.backend, "user", "bob@example.com", "editor")
	moved := app.secretOn(bob, []string{tr.backend}, "read")
	gone := app.patOn(bob, []string{tr.api}, "read")
	goneSecret := gone.json(t)["secret"].(string)

	eq(t, app.send("POST", nodePath(tr.backend)+"/move", tr.admin, obj{"parent_id": tr.globex}).status, 200)
	eq(t, app.bearer("GET", nodePath(tr.backend), moved, nil).json(t)["access"], any("read"))
	eq(t, app.bearer("GET", nodePath(tr.globex), moved, nil).json(t)["access"], any("navigate"))
	eq(t, app.bearer("GET", nodePath(tr.acme), moved, nil).status, 404)

	eq(t, app.call("DELETE", nodePath(tr.api), tr.admin).status, 204)
	contains(t, app.bearer("GET", "/api/v1/catalog/nodes", goneSecret, nil).text(), `"items":[]`)
	eq(t, app.bearer("GET", nodePath(tr.secret), goneSecret, nil).status, 404, "the token does not widen")
	node := at(tokenByID(t, app.get(tokensPath, bob), idOf(gone.json(t))), "nodes", 0).(obj)
	eq(t, jsonText(node), fmt.Sprintf(`{"id":%q,"kind":null,"name":null,"path":null}`, tr.api))
}

func TestTokenNodesLimitSearchAndMCP(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "# API\nThe cache lives here."})
	d.fake.Put(forgefake.Repo{ID: 2, Path: "acme-inc/web", Files: map[string]string{"README.md": "# Web\nThe cache of the web."},
		Branches: []forgefake.Branch{{Name: "main", SHA: sha(2)}}})
	if run := d.app.syncNow(d.admin, d.org, d.conn); run["status"] != "succeeded" {
		t.Fatalf("sync: %v", run)
	}
	d.app.collect()
	bob := bobSession(d.app)
	d.app.grant(d.admin, d.org, "user", "bob@example.com", "viewer")

	all := d.app.get(searchPath("cache"), bob)
	eq(t, len(list(t, all.json(t), "items")), 2, all.text())
	secret := d.app.secretOn(bob, []string{d.id}, "read", "mcp")
	r := d.app.bearer("GET", searchPath("cache"), secret, nil)
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	eq(t, len(items), 1, r.text())
	eq(t, at(items[0], "project", "path"), any("acme/api"))

	out, isErr := call(t, d.app.mcpSession(secret), "list_projects", map[string]any{})
	eq(t, isErr, false, out)
	contains(t, out, `"path": "acme/api"`)
	eq(t, strings.Contains(out, "acme/web"), false, out)
}

func TestTokenNodesOfASuperadminAdministerTheSubtrees(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	tr := newTree(app)
	frontend := app.nodeID(tr.admin, "folder", tr.acme, "frontend")
	root := app.user("chief@example.com", true)
	session := app.session("chief@example.com", password)
	secret := app.secretOn(session, []string{tr.backend}, "read", "write")

	eq(t, app.bearer("PATCH", nodePath(tr.api), secret, obj{"name": "API"}).status, 200)
	backend := app.bearer("GET", nodePath(tr.backend), secret, nil).json(t)
	eq(t, backend["access"], any("read"))
	contains(t, jsonText(backend["permissions"]), `"catalog.access"`)
	app.user("bob@example.com", false)
	r := app.bearer("POST", nodePath(tr.backend)+"/bindings", secret, obj{"subject_kind": "user", "subject": "bob@example.com", "role": "viewer"})
	eq(t, r.status, 200, r.text())
	eq(t, app.bearer("GET", nodePath(frontend), secret, nil).status, 404)
	eq(t, app.bearer("GET", nodePath(tr.globex), secret, nil).status, 404)
	eq(t, app.bearer("GET", nodePath(tr.acme), secret, nil).json(t)["access"], any("navigate"))
	r = app.bearer("POST", "/api/v1/catalog/nodes", secret, obj{"kind": "organization", "slug": "initech", "name": "Initech"})
	eq(t, r.status, 403, r.text())
	r = app.bearer("GET", "/api/v1/users", secret, nil)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	r = app.bearer("GET", "/api/v1/secrets", secret, nil)
	eq(t, code(t, r), any("auth.insufficient_scope"))

	eq(t, app.send("PATCH", "/api/v1/users/"+root.ID.String(), tr.admin, obj{"is_superadmin": false}).status, 200)
	eq(t, app.bearer("GET", nodePath(tr.backend), secret, nil).status, 404)
}
