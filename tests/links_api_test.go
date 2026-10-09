package tests

import (
	"strings"
	"testing"
)

type linkTree struct {
	app                         *testApp
	root                        string
	org, folder, project, other string
	key                         string
}

func newLinkTree(t *testing.T) linkTree {
	t.Helper()
	app := startApp(t)
	root := app.admin()
	org := app.nodeID(root, "organization", "", "acme")
	folder := app.nodeID(root, "folder", org, "backend")
	p := app.node(root, "project", folder, "api")
	other := app.nodeID(root, "project", org, "web")
	r := app.send("PATCH", nodePath(idOf(p)), root, obj{"name": "Billing API"})
	eq(t, r.status, 200, r.text())
	return linkTree{app: app, root: root, org: org, folder: folder, project: idOf(p), other: other, key: at(p, "key", "secret").(string)}
}

func keysOf(items []any) string {
	var out []string
	for _, it := range items {
		l := it.(obj)
		s := l["link_key"].(string)
		if env, ok := l["environment"].(string); ok {
			s += "@" + env
		}
		if u, ok := l["url"].(string); ok {
			s += "=" + u
		} else {
			s += "=null"
		}
		out = append(out, s)
	}
	return strings.Join(out, " ")
}

func TestTemplatesAreInheritedOverriddenAndDisabled(t *testing.T) {
	t.Parallel()
	tr := newLinkTree(t)
	app, root := tr.app, tr.root
	app.putTemplate(root, tr.org, "grafana", "https://grafana.example/d/{project.slug}")
	app.putTemplate(root, tr.org, "sentry", "https://sentry.example/{project.slug}")
	r := app.send("PUT", templatesPath(tr.org)+"/grafana-business", root,
		obj{"kind_key": "grafana", "template": "https://biz.example/{path|raw}", "position": 5})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["kind_key"], any("grafana"))

	items := app.links(root, tr.project, "")
	eq(t, keysOf(items), "grafana=https://grafana.example/d/api grafana-business=https://biz.example/acme/backend/api sentry=https://sentry.example/api")
	first := items[0].(obj)
	eq(t, first["node_id"], any(tr.org))
	eq(t, first["inherited"], any(true))
	eq(t, first["kind_key"], any("grafana"))
	eq(t, first["check"], nil)
	eq(t, len(list(t, first, "missing")), 0)

	app.putTemplate(root, tr.folder, "grafana", "https://grafana.example/folder/{project.slug}")
	r = app.send("PUT", templatesPath(tr.folder)+"/sentry", root, obj{"kind_key": "sentry", "disabled": true})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["template"], nil)
	eq(t, keysOf(app.links(root, tr.project, "")), "grafana=https://grafana.example/folder/api grafana-business=https://biz.example/acme/backend/api")
	eq(t, keysOf(app.links(root, tr.other, "")), "grafana=https://grafana.example/d/web grafana-business=https://biz.example/acme/web sentry=https://sentry.example/web")

	r = app.get(templatesPath(tr.folder), root)
	eq(t, r.status, 200, r.text())
	tpls := list(t, r.json(t), "items")
	eq(t, len(tpls), 3)
	byKey := map[string]obj{}
	for _, it := range tpls {
		byKey[it.(obj)["link_key"].(string)] = it.(obj)
	}
	eq(t, byKey["grafana"]["inherited"], any(false))
	eq(t, byKey["grafana-business"]["node_id"], any(tr.org))
	eq(t, byKey["grafana-business"]["inherited"], any(true))
	eq(t, byKey["sentry"]["disabled"], any(true))

	r = app.call("DELETE", templatesPath(tr.folder)+"/grafana-business", root)
	eq(t, r.status, 404)
	eq(t, code(t, r), any("not_found"))
	eq(t, app.call("DELETE", templatesPath(tr.folder)+"/sentry", root).status, 204)
	eq(t, len(app.links(root, tr.project, "")), 3)

	eq(t, app.call("DELETE", nodePath(tr.other), root).status, 204)
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM link_templates WHERE node_id = $1", tr.other), int64(0))
}

func TestTemplateErrors(t *testing.T) {
	t.Parallel()
	tr := newLinkTree(t)
	app, root := tr.app, tr.root
	for _, tt := range []struct {
		body obj
		code string
	}{
		{obj{"kind_key": "grafana", "template": "https://x.example/{team}"}, "validation.unknown_variable"},
		{obj{"kind_key": "grafana", "template": "https://x.example/{path"}, "validation.invalid_template"},
		{obj{"kind_key": "grafana", "template": "https://x.example/{path|bold}"}, "validation.invalid_template"},
		{obj{"kind_key": "grafana", "template": "{path|raw}"}, "validation.invalid_link_url"},
		{obj{"kind_key": "grafana", "template": "https://x.example/", "disabled": true}, "validation.invalid_template"},
		{obj{"kind_key": "grafana"}, "validation.invalid_template"},
		{obj{"kind_key": "nope", "template": "https://x.example/"}, "validation.unknown_link_kind"},
		{obj{"kind_key": "grafana", "template": "https://x.example/", "position": -1}, "validation.invalid_position"},
		{obj{"template": "https://x.example/"}, "validation.invalid_body"},
	} {
		r := app.send("PUT", templatesPath(tr.org)+"/grafana", root, tt.body)
		eq(t, r.status, 400, jsonText(tt.body))
		eq(t, code(t, r), any(tt.code), jsonText(tt.body))
	}
	r := app.send("PUT", templatesPath(tr.org)+"/grafana", root, obj{"kind_key": "grafana", "template": "https://x.example/{team}"})
	if !strings.Contains(r.json(t)["message"].(string), "character 20") {
		t.Fatalf("message names the position: %s", r.body)
	}
	eq(t, code(t, app.send("PUT", templatesPath(tr.org)+"/Bad_Key", root, obj{"kind_key": "grafana", "template": "https://x.example/"})),
		any("validation.invalid_link_kind_key"))
	eq(t, app.count("link_templates"), int64(0))

	for i := range 50 {
		app.putTemplate(root, tr.org, "logs", "https://x.example/")
		r := app.send("PUT", templatesPath(tr.org)+"/k"+string(rune('a'+i/26))+string(rune('a'+i%26)), root,
			obj{"kind_key": "logs", "template": "https://x.example/"})
		if i < 49 {
			eq(t, r.status, 200, i)
		} else {
			eq(t, r.status, 400)
			eq(t, code(t, r), any("validation.too_many_link_templates"))
		}
	}
}

func TestPreviewExpandsWithoutSaving(t *testing.T) {
	t.Parallel()
	tr := newLinkTree(t)
	app, root := tr.app, tr.root
	app.user("vic@example.com", false)
	app.grant(root, tr.folder, "user", "vic@example.com", "viewer")
	vic := app.session("vic@example.com", password)
	preview := templatesPath(tr.folder) + "/preview"
	r := app.send("POST", preview, vic, obj{"template": "https://x.example/{project.slug}", "project_id": tr.project})
	eq(t, r.status, 200, r.text())
	eq(t, keysOf(list(t, r.json(t), "items")), "preview=https://x.example/api")
	eq(t, app.count("link_templates"), int64(0))
	eq(t, app.count("link_targets"), int64(0), "a preview registers no address")

	r = app.send("POST", templatesPath(tr.project)+"/preview", root, obj{"template": "https://x.example/{branch|raw}", "branch": "feature/x"})
	eq(t, keysOf(list(t, r.json(t), "items")), "preview=https://x.example/feature/x")

	r = app.send("POST", preview, vic, obj{"template": "https://x.example/{project.slug}"})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_project"))
	eq(t, app.send("POST", preview, vic, obj{"template": "https://x.example/", "project_id": tr.other}).status, 404, "not in the subtree")
	eq(t, app.send("POST", preview, vic, obj{"template": "https://x.example/", "project_id": newID()}).status, 404)
	eq(t, code(t, app.send("POST", preview, vic, obj{"template": "https://x.example/{team}", "project_id": tr.project})),
		any("validation.unknown_variable"))
	tok := app.pat(vic, "read")
	eq(t, app.bearer("POST", preview, tok, obj{"template": "https://x.example/", "project_id": tr.project}).status, 200)
}

func TestNodeVariables(t *testing.T) {
	t.Parallel()
	tr := newLinkTree(t)
	app, root := tr.app, tr.root
	put := func(node string, vars obj) reply {
		return app.send("PUT", nodePath(node)+"/vars", root, obj{"vars": vars})
	}
	eq(t, put(tr.org, obj{"grafana_org": "1", "team": "core"}).status, 200)
	r := put(tr.folder, obj{"grafana_org": "7", "b": "x"})
	eq(t, r.status, 200, r.text())
	eq(t, len(list(t, r.json(t), "items")), 3)
	app.putTemplate(root, tr.org, "grafana", "https://grafana.example/?orgId={vars.grafana_org}&team={vars.team}")
	eq(t, keysOf(app.links(root, tr.project, "")), "grafana=https://grafana.example/?orgId=7&team=core")
	eq(t, keysOf(app.links(root, tr.other, "")), "grafana=https://grafana.example/?orgId=1&team=core")

	r = put(tr.folder, obj{"b": "y"})
	eq(t, r.status, 200)
	items := list(t, app.get(nodePath(tr.folder)+"/vars", root).json(t), "items")
	eq(t, jsonText(items), jsonText([]obj{
		{"key": "b", "value": "y", "node_id": tr.folder, "inherited": false},
		{"key": "grafana_org", "value": "1", "node_id": tr.org, "inherited": true},
		{"key": "team", "value": "core", "node_id": tr.org, "inherited": true},
	}))
	eq(t, code(t, put(tr.folder, obj{"Bad": "x"})), any("validation.invalid_vars"))
	eq(t, code(t, put(tr.folder, obj{"a": strings.Repeat("x", 1025)})), any("validation.invalid_vars"))
	eq(t, code(t, app.send("PUT", nodePath(tr.folder)+"/vars", root, obj{})), any("validation.invalid_body"))
}

func TestLinksPerEnvironmentBranchAndMissingVariables(t *testing.T) {
	t.Parallel()
	tr := newLinkTree(t)
	app, root := tr.app, tr.root
	app.putTemplate(root, tr.org, "logs", "https://logs.example/?ns={namespace}")
	app.putTemplate(root, tr.org, "runbook", "https://ci.example/{project.slug}/tree/{branch|raw}")

	items := app.links(root, tr.project, "")
	eq(t, keysOf(items), "logs=null runbook=null")
	eq(t, jsonText(items[0].(obj)["missing"]), `["namespace"]`)
	eq(t, items[0].(obj)["check"], nil)
	eq(t, jsonText(items[1].(obj)["missing"]), `["branch"]`)

	app.deployWith(tr.project, tr.key, "k1", "api", "staging", obj{"namespace": "api-stg"})
	app.deployWith(tr.project, tr.key, "k2", "api", "production", obj{"namespace": "api-prod"})
	items = app.links(root, tr.project, "?branch=feature%2Fx")
	eq(t, keysOf(items), "logs@production=https://logs.example/?ns=api-prod logs@staging=https://logs.example/?ns=api-stg "+
		"runbook=https://ci.example/api/tree/feature/x")
	eq(t, items[0].(obj)["service"], any("api"))
	eq(t, keysOf(app.links(root, tr.project, "?environment=production")),
		"logs@production=https://logs.example/?ns=api-prod runbook=null")
	r := app.get(linksPath(tr.project)+"?branch=a%20b", root)
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_branch"))
	eq(t, app.count("link_targets"), int64(3), "expanded addresses are registered")

	eq(t, app.send("PATCH", nodePath(tr.project), root, obj{"default_branch": "main"}).status, 200)
	app.putTemplate(root, tr.org, "docs", "https://docs.example/{path|raw}")
	eq(t, app.send("PATCH", nodePath(tr.folder), root, obj{"slug": "core"}).status, 200)
	items = app.links(root, tr.project, "?environment=staging")
	eq(t, keysOf(items), "logs@staging=https://logs.example/?ns=api-stg runbook=https://ci.example/api/tree/main docs=https://docs.example/acme/core/api")

	eq(t, app.get(linksPath(tr.folder), root).status, 404)
	eq(t, app.get(linksPath(newID()), root).status, 404)
}

func TestLinkRightsAndScopes(t *testing.T) {
	t.Parallel()
	tr := newLinkTree(t)
	app, root := tr.app, tr.root
	app.putTemplate(root, tr.org, "grafana", "https://grafana.example/")
	app.user("vic@example.com", false)
	app.user("eve@example.com", false)
	app.grant(root, tr.org, "user", "vic@example.com", "viewer")
	app.grant(root, tr.folder, "user", "eve@example.com", "editor")
	vic := app.session("vic@example.com", password)
	eve := app.session("eve@example.com", password)

	r := app.send("PUT", templatesPath(tr.org)+"/grafana", vic, obj{"kind_key": "grafana", "template": "https://x.example/"})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	eq(t, code(t, app.send("PUT", nodePath(tr.org)+"/vars", vic, obj{"vars": obj{}})), any("auth.forbidden"))
	eq(t, app.get(templatesPath(tr.org), vic).status, 200)
	eq(t, len(app.links(vic, tr.project, "")), 1)

	eq(t, app.send("PUT", templatesPath(tr.folder)+"/grafana", eve, obj{"kind_key": "grafana", "template": "https://x.example/"}).status, 200)
	eq(t, app.send("PUT", templatesPath(tr.org)+"/grafana", eve, obj{"kind_key": "grafana", "template": "https://x.example/"}).status, 403)
	eq(t, app.get(templatesPath(tr.other), eve).status, 404, "invisible")

	read := app.pat(eve, "read")
	write := app.pat(eve, "write")
	eq(t, app.bearer("GET", linksPath(tr.project), read, nil).status, 200)
	r = app.bearer("PUT", templatesPath(tr.folder)+"/grafana", read, obj{"kind_key": "grafana", "template": "https://x.example/"})
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, app.bearer("PUT", templatesPath(tr.folder)+"/grafana", write, obj{"kind_key": "grafana", "template": "https://y.example/"}).status, 200)
	eq(t, code(t, app.bearer("GET", templatesPath(tr.folder), write, nil)), any("auth.insufficient_scope"))
	eq(t, app.call("GET", linksPath(tr.project), "").status, 401)

	r = app.call("POST", templatesPath(tr.folder), root)
	eq(t, r.status, 405)
	eq(t, r.header("Allow"), "GET,HEAD")
}
