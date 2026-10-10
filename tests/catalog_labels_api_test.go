package tests

import (
	"net/url"
	"testing"
)

func labelsPath(query string) string { return "/api/v1/catalog/labels" + query }

func TestLabelSuggestionsFollowVisibility(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	hidden := app.nodeID(root, "organization", "", "hidden")
	label := func(node string, labels obj) {
		r := app.send("PATCH", nodePath(node), root, obj{"labels": labels})
		eq(t, r.status, 200, r.text())
	}
	label(app.nodeID(root, "project", acme, "pay"), obj{"team": "payments", "tier": "gold"})
	label(app.nodeID(root, "project", acme, "core"), obj{"team": "core"})
	label(app.nodeID(root, "project", acme, "web"), obj{"team": "core", "under_score": ""})
	label(app.nodeID(root, "project", hidden, "x"), obj{"secret-project": "x", "team": "ghost"})
	app.user("viewer@example.com", false)
	app.grant(root, acme, "user", "viewer@example.com", "viewer")
	viewer := app.session("viewer@example.com", password)

	r := app.get(labelsPath("?q=te"), viewer)
	eq(t, r.status, 200, r.text())
	eq(t, jsonText(r.json(t)["items"]), `[{"count":3,"key":"team"}]`)
	r = app.get(labelsPath("?key=team"), viewer)
	eq(t, jsonText(r.json(t)["items"]), `[{"count":2,"value":"core"},{"count":1,"value":"payments"}]`)
	eq(t, jsonText(app.get(labelsPath("?key=team&q=PAY"), viewer).json(t)["items"]), `[{"count":1,"value":"payments"}]`)
	eq(t, jsonText(app.get(labelsPath("?q=secret"), viewer).json(t)["items"]), `[]`, "labels of invisible nodes")
	eq(t, jsonText(app.get(labelsPath("?q="+url.QueryEscape("under_")), viewer).json(t)["items"]), `[{"count":1,"key":"under_score"}]`)
	eq(t, jsonText(app.get(labelsPath("?q="+url.QueryEscape("%")), viewer).json(t)["items"]), `[]`, "wildcards are literal")
	eq(t, jsonText(app.get(labelsPath("?key=under_score"), viewer).json(t)["items"]), `[{"count":1,"value":""}]`)
	keys := list(t, app.get(labelsPath(""), root).json(t), "items")
	eq(t, at(keys[0].(obj), "key"), any("team"))
	eq(t, at(keys[0].(obj), "count"), any(float64(4)), "the superadmin sees every node")
	eq(t, code(t, app.get(labelsPath("?key="), viewer)), any("validation.invalid_filter"))
	eq(t, app.bearer("GET", labelsPath("?q=t"), app.pat(viewer, "read"), nil).status, 200)
	eq(t, app.get(labelsPath(""), "").status, 401)
}
