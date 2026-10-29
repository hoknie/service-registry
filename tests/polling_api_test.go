package tests

import "testing"

func pollingPath(id string) string { return nodePath(id) + "/polling" }

func TestPollingSettingsInheritPerField(t *testing.T) {
	t.Parallel()
	app := startApp(t, "HTTP_CLIENT_TIMEOUT_SECS", "30")
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	payments := app.nodeID(root, "folder", acme, "payments")
	api := app.nodeID(root, "project", payments, "api")

	r := app.get(pollingPath(api), root)
	eq(t, r.status, 200, r.text())
	body := r.json(t)
	eq(t, body["interval_secs"], any(nil))
	eq(t, body["timeout_secs"], any(float64(30)))
	eq(t, body["mode"], any("on"))
	eq(t, jsonText(body["own"]), `{}`)
	eq(t, jsonText(body["from"]), `{"interval_secs":null,"mode":null,"timeout_secs":null}`)
	eq(t, at(body, "defaults", "timeout_secs"), any(float64(30)))
	eq(t, at(body, "defaults", "repo_sync_interval_secs"), any(float64(app.cfg.Jobs.RepoSyncIntervalSecs)))
	eq(t, at(body, "defaults", "docs_interval_secs"), any(float64(app.cfg.Knowledge.IntervalSecs)))
	eq(t, at(body, "defaults", "skills_interval_secs"), any(float64(app.cfg.Skills.SyncIntervalSecs)))

	eq(t, app.send("PUT", pollingPath(acme), root, obj{"interval_secs": 1800}).status, 200)
	eq(t, app.send("PUT", pollingPath(payments), root, obj{"mode": "off"}).status, 200)
	body = app.get(pollingPath(api), root).json(t)
	eq(t, body["interval_secs"], any(float64(1800)))
	eq(t, at(body, "from", "interval_secs", "name"), any("acme"))
	eq(t, body["mode"], any("off"))
	eq(t, at(body, "from", "mode", "name"), any("payments"))

	r = app.send("PUT", pollingPath(api), root, obj{"mode": "on", "timeout_secs": 5})
	eq(t, r.status, 200, r.text())
	body = r.json(t)
	eq(t, body["mode"], any("on"))
	eq(t, body["timeout_secs"], any(float64(5)))
	eq(t, jsonText(body["own"]), `{"mode":"on","timeout_secs":5}`)
	eq(t, at(body, "from", "mode"), any(nil))

	r = app.send("PUT", pollingPath(api), root, obj{})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["mode"], any("off"))
	eq(t, jsonText(r.json(t)["own"]), `{}`)
}

func TestPollingSettingsRefuseBadValuesAndRights(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	for _, body := range []obj{{"interval_secs": 30}, {"interval_secs": 86401}, {"timeout_secs": 0}, {"timeout_secs": 601},
		{"mode": "sometimes"}, {"interval": 600}, {"interval_secs": "600"}} {
		r := app.send("PUT", pollingPath(acme), root, body)
		eq(t, r.status, 400, jsonText(body))
		eq(t, code(t, r), any("validation.invalid_polling"))
	}
	eq(t, jsonText(app.get(pollingPath(acme), root).json(t)["own"]), `{}`)

	app.user("viewer@example.com", false)
	app.grant(root, acme, "user", "viewer@example.com", "viewer")
	viewer := app.session("viewer@example.com", password)
	eq(t, app.get(pollingPath(acme), viewer).status, 200)
	r := app.send("PUT", pollingPath(acme), viewer, obj{"mode": "off"})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	app.user("stranger@example.com", false)
	eq(t, app.get(pollingPath(acme), app.session("stranger@example.com", password)).status, 404)

	read := app.pat(root, "read")
	eq(t, app.bearer("GET", pollingPath(acme), read, nil).status, 200)
	eq(t, code(t, app.bearer("PUT", pollingPath(acme), read, obj{"mode": "off"})), any("auth.insufficient_scope"))
	eq(t, app.call("POST", pollingPath(acme), root).status, 405)
}
