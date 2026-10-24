package tests

import (
	"testing"
)

type topLevelWorld struct {
	f                     rightsFixture
	acme, globex, backend string
	api                   string
}

func newTopLevelWorld(t *testing.T) topLevelWorld {
	t.Helper()
	f := newRightsFixture(t)
	w := topLevelWorld{f: f}
	w.acme = f.app.nodeID(f.admin, "organization", "", "acme")
	w.globex = f.app.nodeID(f.admin, "organization", "", "globex")
	w.backend = f.app.nodeID(f.admin, "folder", w.acme, "backend")
	w.api = f.app.nodeID(f.admin, "project", w.backend, "api")
	return w
}

func (w topLevelWorld) top(t *testing.T, get func(string) reply) string {
	t.Helper()
	r := get("/api/v1/catalog/nodes")
	eq(t, r.status, 200, r.text())
	out := ""
	for _, n := range list(t, r.json(t), "items") {
		if out != "" {
			out += ","
		}
		out += n.(obj)["slug"].(string) + ":" + n.(obj)["access"].(string)
	}
	return out
}

func TestCatalogReadShowsTheTopLevelAsStubs(t *testing.T) {
	t.Parallel()
	w := newTopLevelWorld(t)
	app, bob := w.f.app, w.f.bob
	session := func(path string) reply { return app.get(path, bob) }
	eq(t, w.top(t, session), "")

	w.f.grant(t, obj{"catalog": []string{"read"}})
	eq(t, w.top(t, session), "acme:navigate,globex:navigate")
	acme := app.get(nodePath(w.acme), bob)
	eq(t, acme.status, 200)
	eq(t, at(acme.json(t), "access"), any("navigate"))
	eq(t, has(acme.json(t), "description"), false)
	eq(t, jsonText(at(acme.json(t), "permissions")), `[]`)
	children := app.get("/api/v1/catalog/nodes?parent="+w.acme, bob)
	eq(t, children.status, 200, children.text())
	eq(t, len(list(t, children.json(t), "items")), 0)
	eq(t, jsonText(app.get(nodePath(w.acme)+"/activity", bob).json(t)), `{"processes":[],"summary":{"failed":0,"queued":0,"running":0}}`)
	for _, hidden := range []string{nodePath(w.backend), nodePath(w.api), nodePath(w.api) + "/owners"} {
		r := app.get(hidden, bob)
		if r.status != 404 && r.status != 403 {
			t.Errorf("%s: %d %s", hidden, r.status, r.body)
		}
	}
	found := app.get("/api/v1/catalog/table?q=glob", bob)
	eq(t, found.status, 200, found.text())
	eq(t, slugs(t, found.json(t), "items"), "globex")
	eq(t, app.get("/api/v1/catalog/table?q=api", bob).json(t)["total"], any(float64(0)))
	eq(t, app.send("PATCH", nodePath(w.globex), bob, obj{"name": "G"}).status, 403)

	app.grant(w.f.admin, w.acme, "user", "bob@example.com", "viewer")
	eq(t, w.top(t, session), "acme:read,globex:navigate")
}

func TestCatalogReadThroughTokens(t *testing.T) {
	t.Parallel()
	w := newTopLevelWorld(t)
	app, bob := w.f.app, w.f.bob
	w.f.grant(t, obj{"catalog": []string{"read"}})
	app.grant(w.f.admin, w.api, "user", "bob@example.com", "viewer")
	for _, tt := range []struct {
		name  string
		token string
		want  string
	}{
		{"read scope", app.pat(bob, "read"), "acme:navigate,globex:navigate"},
		{"read scope on api", app.secretOn(bob, []string{w.api}, "read"), "acme:navigate"},
	} {
		tok := tt.token
		got := w.top(t, func(path string) reply { return app.bearer("GET", path, tok, nil) })
		if got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
	mcp := app.pat(bob, "mcp")
	r := app.bearer("GET", "/api/v1/catalog/nodes", mcp, nil)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))

	refused := app.issue(bob, obj{"name": "root", "scopes": []string{"read", "catalog.root"}, "expires_in_days": 5})
	eq(t, refused.status, 400, refused.text())
	eq(t, code(t, refused), any("validation.invalid_scopes"))
}
