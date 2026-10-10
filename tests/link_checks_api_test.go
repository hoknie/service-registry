package tests

import (
	"testing"
)

func TestCheckOnRequest(t *testing.T) {
	t.Parallel()
	tr := newLinkTree(t)
	app, root := tr.app, tr.root
	app.allowLoopbackChecks()
	tg := newTarget(t, 200)
	app.putTemplate(root, tr.org, "grafana", tg.URL+"/d/{project.slug}")
	app.putTemplate(root, tr.org, "logs", "https://logs.example/?ns={namespace}")
	app.user("vic@example.com", false)
	app.grant(root, tr.project, "user", "vic@example.com", "viewer")
	vic := app.session("vic@example.com", password)

	r := app.call("POST", linksPath(tr.project)+"/grafana/check", vic)
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	eq(t, len(items), 1)
	check := items[0].(obj)["check"].(obj)
	eq(t, check["status"], any("ok"))
	eq(t, check["http_status"], any(float64(200)))
	checkedAt := check["checked_at"]
	eq(t, tg.hits.Load(), int32(1))

	r = app.call("POST", linksPath(tr.project)+"/grafana/check", vic)
	eq(t, r.status, 200)
	eq(t, at(list(t, r.json(t), "items")[0], "check", "checked_at"), checkedAt)
	eq(t, tg.hits.Load(), int32(1))

	eq(t, at(app.links(vic, tr.project, "")[1], "check", "status"), any("ok"))

	r = app.call("POST", linksPath(tr.project)+"/logs/check", vic)
	eq(t, r.status, 200, r.text())
	eq(t, list(t, r.json(t), "items")[0].(obj)["check"], nil)

	eq(t, app.call("POST", linksPath(tr.project)+"/sentry/check", vic).status, 404)
	eq(t, app.call("POST", linksPath(tr.folder)+"/grafana/check", root).status, 404)

	eq(t, app.bearer("POST", linksPath(tr.project)+"/grafana/check", app.pat(vic, "read"), nil).status, 200)
	eq(t, code(t, app.bearer("POST", linksPath(tr.project)+"/grafana/check", app.pat(vic, "write"), nil)), any("auth.insufficient_scope"))
}

func TestCheckHistoryIsCut(t *testing.T) {
	t.Parallel()
	tr := newLinkTree(t)
	app, root := tr.app, tr.root
	app.services.Config.LinkCheck.History = 3
	app.allowLoopbackChecks()
	tg := newTarget(t, 404)
	app.putTemplate(root, tr.org, "docs", tg.URL+"/{project.slug}")
	for range 5 {
		eq(t, app.call("POST", linksPath(tr.project)+"/docs/check", root).status, 200)
		execSQL(t, app.db, "UPDATE link_targets SET last_checked_at = last_checked_at - interval '1 minute'")
	}
	eq(t, tg.hits.Load(), int32(5))
	r := app.get(linksPath(tr.project)+"/docs/checks", root)
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	eq(t, len(items), 3)
	first := items[0].(obj)
	eq(t, first["url"], any(tg.URL+"/api"))
	eq(t, first["status"], any("not_found"))
	eq(t, first["http_status"], any(float64(404)))
	if items[0].(obj)["checked_at"].(string) < items[2].(obj)["checked_at"].(string) {
		t.Fatal("newest first")
	}
	eq(t, app.get(linksPath(tr.project)+"/nope/checks", root).status, 404)
}

func TestDefaultPolicyBlocksLoopback(t *testing.T) {
	t.Parallel()
	tr := newLinkTree(t)
	app, root := tr.app, tr.root
	tg := newTarget(t, 200)
	app.putTemplate(root, tr.org, "grafana", tg.URL+"/")
	r := app.call("POST", linksPath(tr.project)+"/grafana/check", root)
	eq(t, r.status, 200, r.text())
	check := list(t, r.json(t), "items")[0].(obj)["check"].(obj)
	eq(t, check["status"], any("blocked"))
	eq(t, check["http_status"], nil)
	eq(t, tg.hits.Load(), int32(0))
}
