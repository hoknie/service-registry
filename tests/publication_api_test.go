package tests

import (
	"testing"
)

func publicationPath(id string) string { return nodePath(id) + "/publication" }

type publicationWorld struct {
	f                       rightsFixture
	acme, backend, api, sec string
}

func newPublicationWorld(t *testing.T) publicationWorld {
	t.Helper()
	f := newRightsFixture(t)
	w := publicationWorld{f: f}
	w.acme = f.app.nodeID(f.admin, "organization", "", "acme")
	w.backend = f.app.nodeID(f.admin, "folder", w.acme, "backend")
	w.api = f.app.nodeID(f.admin, "project", w.backend, "api")
	w.sec = f.app.nodeID(f.admin, "project", w.backend, "secret")
	f.app.grant(f.admin, w.acme, "user", "bob@example.com", "viewer")
	return w
}

func TestPublicationSettingsInheritAndNeedTheRight(t *testing.T) {
	t.Parallel()
	w := newPublicationWorld(t)
	app, bob := w.f.app, w.f.bob

	r := app.get(publicationPath(w.api), bob)
	eq(t, r.status, 200, r.text())
	eq(t, r.text(), `{"publish":false,"versions":[],"default_branch":true,"current_release":true,"name":null,"host":null,"own":{},`+
		`"from":{"current_release":null,"default_branch":null,"publish":null,"versions":null},"entries":[],"can_edit":false}`)

	r = app.send("PUT", publicationPath(w.acme), bob, obj{"publish": true})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))

	w.f.grant(t, obj{"publication": []string{"update"}})
	r = app.send("PUT", publicationPath(w.acme), bob, obj{"publish": true, "versions": []obj{{"pattern": "v*", "kind": "tag"}}})
	eq(t, r.status, 200, r.text())
	body := r.json(t)
	eq(t, body["publish"], any(true))
	eq(t, jsonText(body["entries"]), `[{"kind":"path","url":"/public/docs/acme"}]`)
	eq(t, body["can_edit"], any(true))

	eq(t, app.send("PUT", publicationPath(w.sec), bob, obj{"publish": false}).status, 200)
	api := app.get(publicationPath(w.api), bob).json(t)
	eq(t, api["publish"], any(true))
	eq(t, jsonText(api["versions"]), `[{"kind":"tag","pattern":"v*"}]`)
	eq(t, at(api, "from", "publish", "name"), any("acme"))
	eq(t, jsonText(api["entries"]), `[{"kind":"path","url":"/public/docs/acme"}]`)
	sec := app.get(publicationPath(w.sec), bob).json(t)
	eq(t, sec["publish"], any(false))
	eq(t, jsonText(sec["entries"]), `[]`)
}

func TestPublicationNamesAndHosts(t *testing.T) {
	t.Parallel()
	w := newPublicationWorld(t)
	app, admin := w.f.app, w.f.admin
	r := app.send("PUT", publicationPath(w.api), admin, obj{"publish": true, "name": "acme-api", "host": "docs.acme.com"})
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(r.json(t)["entries"]), `[{"kind":"path","url":"/public/docs/acme-api"},{"kind":"host","url":"https://docs.acme.com/"}]`)

	for _, body := range []obj{{"name": "Acme-API"}, {"name": "acme"}} {
		r := app.send("PUT", publicationPath(w.sec), admin, body)
		eq(t, r.status, 409, jsonText(body))
		eq(t, code(t, r), any("conflict.publication_name_taken"))
	}
	r = app.send("PUT", publicationPath(w.sec), admin, obj{"host": "docs.acme.com"})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.publication_host_taken"))
	for _, body := range []obj{{"publish": nil}, {"versions": []obj{{"pattern": "v*", "kind": "commit"}}}, {"host": "Docs.Acme.com:443"}} {
		r := app.send("PUT", publicationPath(w.sec), admin, body)
		eq(t, r.status, 400, jsonText(body))
		eq(t, code(t, r), any("validation.invalid_publication"))
	}
	r = app.send("PUT", publicationPath(w.acme), admin, obj{"name": "x"})
	eq(t, code(t, r), any("validation.invalid_publication"))
	eq(t, app.send("PUT", publicationPath(w.api), admin, obj{}).status, 200)
	eq(t, jsonText(app.get(publicationPath(w.api), admin).json(t)["own"]), `{}`)
}

func TestPublicationScopes(t *testing.T) {
	t.Parallel()
	w := newPublicationWorld(t)
	app := w.f.app
	w.f.grant(t, obj{"publication": []string{"update"}})
	both := app.pat(w.f.bob, "read", "publication:write")
	eq(t, app.bearer("PUT", publicationPath(w.backend), both, obj{"publish": true}).status, 200)
	readWrite := app.pat(w.f.bob, "read", "write")
	r := app.bearer("PUT", publicationPath(w.backend), readWrite, obj{"publish": true})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, app.bearer("GET", publicationPath(w.backend), readWrite, nil).status, 200)

	super := app.pat(w.f.admin, "read", "write")
	eq(t, code(t, app.bearer("PUT", publicationPath(w.backend), super, obj{"publish": true})), any("auth.insufficient_scope"))
	super = app.pat(w.f.admin, "read", "publication:write")
	eq(t, app.bearer("PUT", publicationPath(w.backend), super, obj{"publish": true}).status, 200)
}
