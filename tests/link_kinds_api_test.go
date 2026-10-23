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
	eq(t, logs["icon"], any("scroll-text"))
	eq(t, logs["icon_url"], nil)
	eq(t, logs["position"], any(float64(10)))
	var icons []string
	for _, it := range items {
		icons = append(icons, it.(obj)["icon"].(string))
	}
	eq(t, jsonText(icons), `["scroll-text","chart-line","bug","bell-ring","waypoints","book-open-check","book-text"]`)
	eq(t, app.call("GET", linkKindsPath, "").status, 401)
}

func TestSuperadminManagesLinkKinds(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	r := app.send("POST", linkKindsPath, root, obj{"key": " Grafana-Business ", "names": obj{"en": "Grafana Business"}})
	eq(t, r.status, 201, r.text())
	k := r.json(t)
	eq(t, k["key"], any("grafana-business"))
	eq(t, k["icon"], any("link"))
	eq(t, k["icon_url"], nil)
	eq(t, k["position"], any(float64(80)), "appended after the last kind")
	eq(t, at(k, "names", "zh"), any("Grafana Business"))

	r = app.send("POST", linkKindsPath, root, obj{"key": "ci", "names": obj{"ru": "Пайплайн", "en": ""},
		"icon": "git-pull-request", "icon_url": "https://ci.example/favicon.png"})
	eq(t, r.status, 201, r.text())
	k = r.json(t)
	eq(t, jsonText(k["names"]), `{"en":"Пайплайн","es":"Пайплайн","ru":"Пайплайн","zh":"Пайплайн"}`)
	eq(t, k["icon"], any("git-pull-request"))
	eq(t, k["icon_url"], any("https://ci.example/favicon.png"))
	eq(t, k["position"], any(float64(90)))
	r = app.send("PATCH", linkKindsPath+"/ci", root, obj{"icon_url": nil})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["icon_url"], nil)
	eq(t, r.json(t)["icon"], any("git-pull-request"))
	r = app.send("POST", linkKindsPath, root, obj{"key": "spread", "names": obj{"es": "Registros", "ru": "Логи"}})
	eq(t, jsonText(r.json(t)["names"]), `{"en":"Registros","es":"Registros","ru":"Логи","zh":"Registros"}`)
	eq(t, app.call("DELETE", linkKindsPath+"/spread", root).status, 204)
	eq(t, app.call("DELETE", linkKindsPath+"/ci", root).status, 204)

	r = app.send("POST", linkKindsPath, root, obj{"key": "grafana-business", "names": allNames("Again")})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.link_kind_taken"))
	r = app.send("POST", linkKindsPath, root, obj{"key": "x", "names": obj{"en": "  ", "ru": nil}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_link_kind_names"))
	eq(t, code(t, app.send("POST", linkKindsPath, root, obj{"key": "x", "names": obj{"de": "Protokolle"}})), any("validation.invalid_link_kind_names"))
	eq(t, code(t, app.send("POST", linkKindsPath, root, obj{"key": "x", "names": allNames("X"), "icon_url": "http://ci.example/i.png"})), any("validation.invalid_link_icon"))
	eq(t, code(t, app.send("POST", linkKindsPath, root, obj{"key": "-x", "names": allNames("X")})), any("validation.invalid_link_kind_key"))
	eq(t, code(t, app.send("POST", linkKindsPath, root, obj{"key": "x", "names": allNames("X"), "icon": "Chart Line"})), any("validation.invalid_link_icon"))
	eq(t, code(t, app.send("POST", linkKindsPath, root, obj{"key": "x", "names": allNames("X"), "position": 10001})), any("validation.invalid_position"))
	eq(t, code(t, app.send("POST", linkKindsPath, root, obj{"key": "x"})), any("validation.invalid_body"))

	r = app.send("PATCH", linkKindsPath+"/docs", root, obj{"names": obj{"en": "Documentation", "ru": "Документация"}, "position": 0})
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(r.json(t)["names"]), `{"en":"Documentation","es":"Documentation","ru":"Документация","zh":"Documentation"}`)
	eq(t, r.json(t)["icon"], any("book-text"), "unchanged")
	first := list(t, app.get(linkKindsPath, root).json(t), "items")[0].(obj)
	eq(t, first["key"], any("docs"), "ordered by position, then key")
	eq(t, app.send("PATCH", linkKindsPath+"/nope", root, obj{"icon": "book"}).status, 404)

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
	eq(t, code(t, app.send("PATCH", linkKindsPath+"/logs", ann, obj{"icon": "book"})), any("auth.forbidden"))
	eq(t, code(t, app.send("PUT", linkKindsPath, ann, obj{"keys": []string{"logs"}})), any("auth.forbidden"))
	eq(t, code(t, app.call("DELETE", linkKindsPath+"/logs", ann)), any("auth.forbidden"))
}

func TestLinkKindScopesOfTokens(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	read := app.pat(root, "read")
	readWrite := app.pat(root, "read", "write")
	admin := app.pat(root, "settings:write")
	eq(t, app.bearer("GET", linkKindsPath, read, nil).status, 200)
	r := app.bearer("POST", linkKindsPath, readWrite, obj{"key": "x", "names": allNames("X")})
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, code(t, app.bearer("GET", linkKindsPath, admin, nil)), any("auth.insufficient_scope"))
	eq(t, app.bearer("POST", linkKindsPath, admin, obj{"key": "x", "names": allNames("X")}).status, 201)
	eq(t, app.bearer("DELETE", linkKindsPath+"/x", admin, nil).status, 204)
	eq(t, code(t, app.bearer("PUT", linkKindsPath, readWrite, obj{"keys": []string{}})), any("auth.insufficient_scope"))
}

func kindKeys(t *testing.T, items []any) string {
	t.Helper()
	var keys []string
	for _, it := range items {
		keys = append(keys, it.(obj)["key"].(string))
	}
	return jsonText(keys)
}

func TestSuperadminReordersLinkKinds(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	order := []string{"sentry", "logs", "grafana", "alertmanager", "traces", "runbook", "docs"}
	r := app.send("PUT", linkKindsPath, root, obj{"keys": order})
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	eq(t, kindKeys(t, items), jsonText(order))
	for i, it := range items {
		eq(t, it.(obj)["position"], any(float64(10*(i+1))))
	}
	eq(t, kindKeys(t, list(t, app.get(linkKindsPath, root).json(t), "items")), jsonText(order))

	r = app.send("PUT", linkKindsPath, root, obj{"keys": []string{"logs", "logs"}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_link_kind_order"))
	eq(t, code(t, app.send("PUT", linkKindsPath, root, obj{"keys": []any{"logs", 1}})), any("validation.invalid_body"))
	eq(t, code(t, app.send("PUT", linkKindsPath, root, obj{})), any("validation.invalid_body"))

	eq(t, app.send("POST", linkKindsPath, root, obj{"key": "ci", "names": obj{"en": "CI"}}).status, 201)
	r = app.send("PUT", linkKindsPath, root, obj{"keys": order})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.link_kind_order_stale"))
	eq(t, kindKeys(t, list(t, app.get(linkKindsPath, root).json(t), "items")), jsonText(append(append([]string{}, order...), "ci")))
	r = app.send("PUT", linkKindsPath, root, obj{"keys": append([]string{"ci"}, order[:6]...)})
	eq(t, r.status, 409, "a missing key is stale too")
}
