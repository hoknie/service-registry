package tests

import (
	"testing"
)

func visibilityPath(id string) string { return nodePath(id) + "/visibility" }

func (w topLevelWorld) open(t *testing.T, id string) {
	t.Helper()
	r := w.f.app.send("PUT", visibilityPath(id), w.f.admin, obj{"visibility": "open"})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["visibility"], any("open"))
}

func TestOpenOrganizationIsReadableBySignedInUsers(t *testing.T) {
	t.Parallel()
	w := newTopLevelWorld(t)
	app, bob := w.f.app, w.f.bob
	session := func(path string) reply { return app.get(path, bob) }
	eq(t, app.get(nodePath(w.acme), w.f.admin).json(t)["visibility"], any("closed"))
	eq(t, w.top(t, session), "")

	w.open(t, w.acme)
	eq(t, w.top(t, session), "acme:read")
	acme := app.get(nodePath(w.acme), bob)
	eq(t, acme.status, 200, acme.text())
	eq(t, jsonText(at(acme.json(t), "permissions")), `["catalog.read"]`)
	eq(t, acme.json(t)["visibility"], any("open"))
	api := app.get(nodePath(w.api), bob)
	eq(t, api.status, 200, api.text())
	eq(t, jsonText(at(api.json(t), "path")) != "", true)
	eq(t, has(api.json(t), "visibility"), false)
	eq(t, app.get(nodePath(w.api)+"/owners", bob).status, 200)
	eq(t, app.get(nodePath(w.globex), bob).status, 404)
	eq(t, slugs(t, app.get("/api/v1/catalog/table?q=api", bob).json(t), "items"), "acme,backend,api")

	eq(t, app.send("PATCH", nodePath(w.api), bob, obj{"name": "API"}).status, 403)
	eq(t, app.send("PUT", visibilityPath(w.acme), bob, obj{"visibility": "closed"}).status, 403)

	app.grant(w.f.admin, w.backend, "user", "bob@example.com", "editor")
	eq(t, jsonText(at(app.get(nodePath(w.api), bob).json(t), "permissions")), `["catalog.read","catalog.write","catalog.keys"]`)
	eq(t, jsonText(at(app.get(nodePath(w.acme), bob).json(t), "permissions")), `["catalog.read"]`)

	robot := idOf(app.send("POST", "/api/v1/users", w.f.admin, obj{"email": "robot@example.com", "display_name": "Robot", "is_service": true}).json(t))
	issued := app.send("POST", "/api/v1/users/"+robot+"/tokens", w.f.admin, obj{"name": "robot", "scopes": []string{"read"}, "expires_in_days": 5})
	eq(t, issued.status, 201, issued.text())
	tok := issued.json(t)["secret"].(string)
	eq(t, w.top(t, func(path string) reply { return app.bearer("GET", path, tok, nil) }), "acme:read")

	r := app.send("PUT", visibilityPath(w.acme), w.f.admin, obj{"visibility": "closed"})
	eq(t, r.status, 200, r.text())
	eq(t, w.top(t, func(path string) reply { return app.bearer("GET", path, tok, nil) }), "")
}

func TestOpenOrganizationThroughTokens(t *testing.T) {
	t.Parallel()
	w := newTopLevelWorld(t)
	app, bob := w.f.app, w.f.bob
	globexPrj := app.nodeID(w.f.admin, "project", w.globex, "billing")
	app.grant(w.f.admin, globexPrj, "user", "bob@example.com", "viewer")
	w.open(t, w.acme)
	for _, tt := range []struct {
		name  string
		token string
		want  string
	}{
		{"read scope", app.pat(bob, "read"), "acme:read,globex:navigate"},
		{"nodes outside acme", app.secretOn(bob, []string{globexPrj}, "read"), "globex:navigate"},
		{"nodes inside acme", app.secretOn(bob, []string{w.backend}, "read"), "acme:navigate"},
	} {
		tok := tt.token
		got := w.top(t, func(path string) reply { return app.bearer("GET", path, tok, nil) })
		if got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
	inside := app.secretOn(bob, []string{w.backend}, "read")
	eq(t, app.bearer("GET", nodePath(w.api), inside, nil).status, 200)
	outside := app.secretOn(bob, []string{globexPrj}, "read")
	eq(t, app.bearer("GET", nodePath(w.api), outside, nil).status, 404)
}

func TestOrganizationVisibilityIsSwitchedByAdminsAndCatalogRight(t *testing.T) {
	t.Parallel()
	w := newTopLevelWorld(t)
	app, bob := w.f.app, w.f.bob
	app.user("carol@example.com", false)
	carol := app.session("carol@example.com", password)
	app.grant(w.f.admin, w.acme, "user", "carol@example.com", "editor")
	r := app.send("PUT", visibilityPath(w.acme), carol, obj{"visibility": "open"})
	eq(t, r.status, 403, r.text())
	eq(t, code(t, r), any("auth.forbidden"))
	eq(t, app.send("PUT", visibilityPath(w.globex), carol, obj{"visibility": "open"}).status, 404)

	app.grant(w.f.admin, w.acme, "user", "carol@example.com", "admin")
	r = app.send("PUT", visibilityPath(w.acme), carol, obj{"visibility": "open"})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["visibility"], any("open"))

	eq(t, app.send("PUT", visibilityPath(w.globex), bob, obj{"visibility": "open"}).status, 404)
	w.f.grant(t, obj{"catalog": []string{"update"}})
	r = app.send("PUT", visibilityPath(w.globex), bob, obj{"visibility": "open"})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["visibility"], any("open"))
	eq(t, app.send("PUT", visibilityPath(w.backend), bob, obj{"visibility": "open"}).status, 403)

	for _, tt := range []struct {
		name string
		id   string
		body obj
		code string
	}{
		{"folder", w.backend, obj{"visibility": "open"}, "validation.visibility_top_level"},
		{"unknown value", w.acme, obj{"visibility": "public"}, "validation.invalid_visibility"},
		{"missing value", w.acme, obj{}, "validation.invalid_visibility"},
	} {
		r := app.send("PUT", visibilityPath(tt.id), w.f.admin, tt.body)
		if r.status != 400 || code(t, r) != any(tt.code) {
			t.Errorf("%s: %d %s", tt.name, r.status, r.body)
		}
	}

	moved := app.send("POST", nodePath(w.acme)+"/move", w.f.admin, obj{"parent_id": w.globex})
	eq(t, moved.status, 200, moved.text())
	eq(t, has(moved.json(t), "visibility"), false)
	back := app.send("POST", nodePath(w.acme)+"/move", w.f.admin, obj{"parent_id": nil})
	eq(t, back.status, 200, back.text())
	eq(t, back.json(t)["visibility"], any("closed"))
}

func TestOpenOrganizationDocumentationIsSearchable(t *testing.T) {
	t.Parallel()
	d := startDocs(t, map[string]string{"README.md": "# API\nThe service keeps an idempotency key for every event."})
	d.app.collect()
	d.app.user("dave@example.com", false)
	dave := d.app.session("dave@example.com", password)
	eq(t, len(list(t, d.app.get(searchPath("idempotency"), dave).json(t), "items")), 0)
	eq(t, d.app.get(docSpacePath("acme/api"), dave).status, 404)

	r := d.app.send("PUT", visibilityPath(d.org), d.admin, obj{"visibility": "open"})
	eq(t, r.status, 200, r.text())
	found := d.app.get(searchPath("idempotency"), dave)
	eq(t, found.status, 200, found.text())
	items := list(t, found.json(t), "items")
	eq(t, len(items), 1)
	eq(t, at(items[0].(obj), "project", "id"), any(d.id))
	space := d.app.get(docSpacePath("acme/api"), dave)
	eq(t, space.status, 200, space.text())
	eq(t, at(space.json(t), "node", "access"), any("read"))
	eq(t, space.json(t)["docs"] != nil, true)
}
