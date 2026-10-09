package tests

import (
	"testing"
)

const environmentsPath = "/api/v1/environments"

func envNames(en string) obj {
	return obj{"en": en, "es": en + " es", "ru": en + " ru", "zh": en + " zh"}
}

func TestEnvironmentDirectory(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	app.user("bob@example.com", false)
	bob := app.session("bob@example.com", password)

	r := app.get(environmentsPath, bob)
	eq(t, r.status, 200, r.text())
	items := list(t, r.json(t), "items")
	eq(t, len(items), 3)
	eq(t, items[0].(obj)["key"], any("production"))
	eq(t, at(items[0], "names", "ru"), any("Продакшн"))
	eq(t, items[2].(obj)["key"], any("development"))
	eq(t, app.call("GET", environmentsPath, "").status, 401)

	r = app.send("POST", environmentsPath, root, obj{"key": " QA ", "names": envNames("QA")})
	eq(t, r.status, 201, r.text())
	eq(t, r.json(t)["key"], any("qa"))
	eq(t, r.json(t)["position"], any(float64(0)))
	r = app.send("POST", environmentsPath, root, obj{"key": "production", "names": envNames("P")})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.environment_taken"))
	eq(t, code(t, app.send("POST", environmentsPath, root, obj{"key": "a b", "names": envNames("X")})), any("validation.invalid_environment_key"))
	eq(t, code(t, app.send("POST", environmentsPath, root, obj{"key": "x", "names": obj{"en": "X"}})), any("validation.invalid_environment_names"))
	eq(t, code(t, app.send("POST", environmentsPath, root, obj{"key": "x", "names": envNames("X"), "position": -1})), any("validation.invalid_position"))

	r = app.send("PATCH", environmentsPath+"/qa", root, obj{"position": 15, "names": envNames("Quality")})
	eq(t, r.status, 200, r.text())
	eq(t, at(r.json(t), "names", "en"), any("Quality"))
	eq(t, list(t, app.get(environmentsPath, root).json(t), "items")[1].(obj)["key"], any("qa"))
	eq(t, app.send("PATCH", environmentsPath+"/nope", root, obj{"position": 1}).status, 404)

	eq(t, code(t, app.send("POST", environmentsPath, bob, obj{"key": "x", "names": envNames("X")})), any("auth.forbidden"))
	eq(t, code(t, app.call("DELETE", environmentsPath+"/qa", bob)), any("auth.forbidden"))

	project, key := app.projectWithKey(root, "api")
	eq(t, app.ingest(project, key, deployed("k1", "api", "staging", "1.0.0", "2026-10-01T10:00:00Z")).status, 201)
	eq(t, app.call("DELETE", environmentsPath+"/staging", root).status, 204)
	eq(t, app.call("DELETE", environmentsPath+"/staging", root).status, 404)
	eq(t, len(list(t, app.get(nodePath(project)+"/environments", root).json(t), "items")), 1)
	eq(t, app.count("service_deployments"), int64(1))
}

func TestEnvironmentScopesOfTokens(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	root := app.admin()
	eq(t, app.bearer("GET", environmentsPath, app.pat(root, "read"), nil).status, 200)
	r := app.bearer("POST", environmentsPath, app.pat(root, "read", "write"), obj{"key": "qa", "names": envNames("QA")})
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, app.bearer("POST", environmentsPath, app.pat(root, "admin"), obj{"key": "qa", "names": envNames("QA")}).status, 201)
	r = app.call("PUT", environmentsPath+"/qa", root)
	eq(t, r.status, 405)
	eq(t, r.header("Allow"), "PATCH,DELETE")
}
