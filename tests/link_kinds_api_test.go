package tests

import (
	"testing"
)

func TestInitialLinkKinds(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	app.user("bob@example.com", false)
	bob := app.session("bob@example.com", password)
	r := app.get(linkKindsPath, bob)
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	eq(t, len(items), 7)
	var keys []string
	for _, it := range items {
		keys = append(keys, it.(obj)["key"].(string))
	}
	eq(t, jsonText(keys), `["logs","grafana","sentry","alertmanager","traces","runbook","docs"]`)
	logs := items[0].(obj)
	eq(t, at(logs, "names", "ru"), any("Логи"))
	eq(t, at(logs, "names", "en"), any("Logs"))
	eq(t, logs["icon"], any("logs"))
	eq(t, logs["position"], any(float64(10)))
	eq(t, app.call("GET", linkKindsPath, "").status, 401)
}

func TestSuperadminManagesLinkKinds(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	r := app.send("POST", linkKindsPath, root, obj{"key": " Grafana-Business ", "names": allNames("Business")})
	eq(t, r.status, 201, r.text())
	k := r.json(t)
	eq(t, k["key"], any("grafana-business"))
	eq(t, k["icon"], any("link"))
	eq(t, k["position"], any(float64(0)))

	r = app.send("POST", linkKindsPath, root, obj{"key": "grafana-business", "names": allNames("Again")})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.link_kind_taken"))
	r = app.send("POST", linkKindsPath, root, obj{"key": "x", "names": obj{"en": "Logs", "ru": "Логи"}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_link_kind_names"))
	eq(t, code(t, app.send("POST", linkKindsPath, root, obj{"key": "-x", "names": allNames("X")})), any("validation.invalid_link_kind_key"))
	eq(t, code(t, app.send("POST", linkKindsPath, root, obj{"key": "x", "names": allNames("X"), "icon": "rocket"})), any("validation.invalid_link_icon"))
	eq(t, code(t, app.send("POST", linkKindsPath, root, obj{"key": "x", "names": allNames("X"), "position": 10001})), any("validation.invalid_position"))
	eq(t, code(t, app.send("POST", linkKindsPath, root, obj{"key": "x"})), any("validation.invalid_body"))

	r = app.send("PATCH", linkKindsPath+"/docs", root, obj{"names": allNames("Documentation"), "position": 0})
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "names", "en"), any("Documentation"))
	eq(t, r.json(t)["icon"], any("docs"), "unchanged")
	first := list(t, app.get(linkKindsPath, root).json(t), "items")[0].(obj)
	eq(t, first["key"], any("docs"), "ordered by position, then key")
	eq(t, app.send("PATCH", linkKindsPath+"/nope", root, obj{"icon": "docs"}).status, 404)

	eq(t, app.call("DELETE", linkKindsPath+"/grafana-business", root).status, 204)
	eq(t, app.call("DELETE", linkKindsPath+"/grafana-business", root).status, 404)

	org := app.nodeID(root, "organization", "", "acme")
	app.putTemplate(root, org, "grafana", "https://grafana.example/")
	r = app.call("DELETE", linkKindsPath+"/grafana", root)
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.link_kind_in_use"))
	eq(t, len(list(t, app.get(linkKindsPath, root).json(t), "items")), 7)

	r = app.call("PUT", linkKindsPath+"/grafana", root)
	eq(t, r.status, 405)
	eq(t, r.header("Allow"), "PATCH,DELETE")
}

func TestOnlyTheSuperadminChangesLinkKinds(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	org := app.nodeID(root, "organization", "", "acme")
	app.user("ann@example.com", false)
	app.grant(root, org, "user", "ann@example.com", "admin")
	ann := app.session("ann@example.com", password)
	r := app.send("POST", linkKindsPath, ann, obj{"key": "x", "names": allNames("X")})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	eq(t, code(t, app.send("PATCH", linkKindsPath+"/logs", ann, obj{"icon": "docs"})), any("auth.forbidden"))
	eq(t, code(t, app.call("DELETE", linkKindsPath+"/logs", ann)), any("auth.forbidden"))
}

func TestLinkKindScopesOfTokens(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	read := app.pat(root, "read")
	readWrite := app.pat(root, "read", "write")
	admin := app.pat(root, "admin")
	eq(t, app.bearer("GET", linkKindsPath, read, nil).status, 200)
	r := app.bearer("POST", linkKindsPath, readWrite, obj{"key": "x", "names": allNames("X")})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, code(t, app.bearer("GET", linkKindsPath, admin, nil)), any("auth.insufficient_scope"))
	eq(t, app.bearer("POST", linkKindsPath, admin, obj{"key": "x", "names": allNames("X")}).status, 201)
	eq(t, app.bearer("DELETE", linkKindsPath+"/x", admin, nil).status, 204)
}
