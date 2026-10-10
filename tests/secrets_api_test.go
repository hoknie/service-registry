package tests

import (
	"testing"
)

func TestSecretsAreInheritedDownTheTree(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	backend := app.nodeID(root, "folder", acme, "backend")
	api := app.nodeID(root, "project", backend, "api")
	org := app.secret(root, acme, obj{"name": "gitlab", "description": "CI token", "value": "glpat-org"})
	folder := app.secret(root, backend, obj{"name": "gitlab", "value": "glpat-folder"})
	global := app.secret(root, "", obj{"name": "github", "ref": "env:GH_TOKEN"})

	r := app.get(secretsPath(api), root)
	eq(t, r.status, 200, r.text())
	lacks(t, r.text(), "glpat-")
	items := list(t, r.json(t), "items")
	eq(t, len(items), 3)
	byID := map[string]obj{}
	for _, it := range items {
		byID[it.(obj)["id"].(string)] = it.(obj)
	}
	eq(t, byID[global]["from"], nil)
	eq(t, byID[global]["storage"], any("reference"))
	eq(t, byID[global]["ref"], any("env:GH_TOKEN"))
	eq(t, at(byID[org], "from", "name"), any("acme"))
	eq(t, byID[org]["description"], any("CI token"))
	eq(t, byID[org]["storage"], any("stored"))
	eq(t, len(byID[org]["fingerprint"].(string)), 4)
	eq(t, at(byID[folder], "from", "name"), any("backend"))
	eq(t, byID[org]["own"], any(false))

	own := list(t, app.get(secretsPath(backend), root).json(t), "items")
	owned := 0
	for _, it := range own {
		if it.(obj)["own"] == true {
			owned++
		}
	}
	eq(t, owned, 1)
	eq(t, len(list(t, app.get(secretsPath(acme), root).json(t), "items")), 2, "children's secrets are not visible above")
	eq(t, app.send("POST", secretsPath(api), root, obj{"name": "x", "value": "y"}).status, 404, "projects have no secrets of their own")
}

func TestSecretRulesAndRights(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	globex := app.nodeID(root, "organization", "", "globex")
	for email, role := range map[string]string{"viewer@example.com": "viewer", "editor@example.com": "editor", "admin@example.com": "admin"} {
		app.user(email, false)
		app.grant(root, acme, "user", email, role)
	}
	viewer := app.session("viewer@example.com", password)
	editor := app.session("editor@example.com", password)
	admin := app.session("admin@example.com", password)

	eq(t, app.get(secretsPath(acme), viewer).status, 200)
	eq(t, code(t, app.send("POST", secretsPath(acme), editor, obj{"name": "a", "value": "b"})), any("auth.forbidden"))
	created := app.send("POST", secretsPath(acme), admin, obj{"name": " GitLab ", "value": "glpat-1"})
	eq(t, created.status, 201, created.text())
	eq(t, created.json(t)["name"], any("GitLab"))
	eq(t, created.json(t)["own"], any(true))
	lacks(t, created.text(), "glpat-1")
	id := idOf(created.json(t))
	taken := app.send("POST", secretsPath(acme), admin, obj{"name": "gitlab", "value": "x"})
	eq(t, taken.status, 409)
	eq(t, code(t, taken), any("conflict.secret_name_taken"))
	for _, body := range []obj{{"value": "x"}, {"name": "x"}, {"name": "x", "value": "a b"}, {"name": "x", "value": "a", "ref": "env:X"},
		{"name": "x", "ref": "vault:x"}} {
		r := app.send("POST", secretsPath(acme), admin, body)
		eq(t, r.status, 400, jsonText(body))
		eq(t, code(t, r), any("validation.invalid_secret"))
	}

	upd := app.send("PATCH", secretsPath(acme)+"/"+id, admin, obj{"description": "CI", "value": "glpat-2"})
	eq(t, upd.status, 200, upd.text())
	eq(t, upd.json(t)["description"], any("CI"))
	lacks(t, upd.text(), "glpat-2")
	eq(t, app.send("PATCH", secretsPath(globex)+"/"+id, root, obj{"description": "x"}).status, 404, "a secret of another node")
	eq(t, app.bearer("GET", secretsPath(acme), app.pat(viewer, "read"), nil).status, 200)
	eq(t, app.bearer("POST", secretsPath(acme), app.pat(admin, "read"), obj{"name": "y", "value": "z"}).status, 403)
	eq(t, app.bearer("POST", secretsPath(acme), app.pat(admin, "write"), obj{"name": "y", "value": "z"}).status, 201)

	eq(t, code(t, app.get(globalSecretsPath, admin)), any("auth.forbidden"))
	g := app.send("POST", globalSecretsPath, root, obj{"name": "sa", "value": "token"})
	eq(t, g.status, 201, g.text())
	eq(t, g.json(t)["node_id"], nil)
	gid := idOf(g.json(t))
	eq(t, app.get(globalSecretsPath+"/"+gid, root).status, 200)
	eq(t, app.bearer("GET", globalSecretsPath, app.pat(root, "read"), nil).status, 403)
	eq(t, app.bearer("GET", globalSecretsPath, app.pat(root, "admin"), nil).status, 200)
	eq(t, app.get(globalSecretsPath+"/"+id, root).status, 404, "a node secret is not global")

	conn := app.send("POST", connectionsPath(acme), root, obj{"kind": "github", "owner_path": "acme-inc", "credentials": obj{"secret_id": id}})
	eq(t, conn.status, 201, conn.text())
	used := list(t, app.get(secretsPath(acme), root).json(t), "items")
	for _, it := range used {
		if it.(obj)["id"] == id {
			eq(t, it.(obj)["used_by"], any(float64(1)))
		}
	}
	inUse := app.call("DELETE", secretsPath(acme)+"/"+id, admin)
	eq(t, inUse.status, 409)
	eq(t, code(t, inUse), any("conflict.secret_in_use"))
	eq(t, app.call("DELETE", connectionPath(acme, idOf(conn.json(t))), root).status, 204)
	eq(t, app.call("DELETE", secretsPath(acme)+"/"+id, admin).status, 204)
	eq(t, app.call("DELETE", globalSecretsPath+"/"+gid, root).status, 204)
}
