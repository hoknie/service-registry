package tests

import (
	"strings"
	"testing"

	"svc-registry/internal/testsupport/forgefake"
)

func TestConnectionDefaultsAndSecretHandling(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	r := app.send("POST", connectionsPath(acme), root, obj{"kind": "github", "owner_path": " acme-inc ", "credentials": obj{"token": "ghp_supersecret"}})
	eq(t, r.status, 201, r.text())
	c := r.json(t)
	eq(t, c["api_url"], any("https://api.github.com"))
	eq(t, c["owner_path"], any("acme-inc"))
	eq(t, c["interval_secs"], any(float64(900)))
	eq(t, c["mirror_subgroups"], any(false))
	eq(t, at(c, "credentials", "kind"), any("token"))
	eq(t, len(at(c, "credentials", "fingerprint").(string)), 4)
	eq(t, c["webhook"], nil)
	eq(t, c["last_run"], nil)
	lacks(t, r.text(), "ghp_supersecret")
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM forge_connections WHERE position('ghp_supersecret' IN credentials_enc) > 0"), int64(0))
	enc := scalar[string](t, app.db, "SELECT credentials_enc FROM forge_connections")
	eq(t, strings.HasPrefix(enc, "v1.k1."), true, enc)
	lacks(t, app.get(connectionPath(acme, idOf(c)), root).text(), "ghp_")

	other := app.nodeID(root, "organization", "", "other")
	f := forgefake.Start(t, "github", forgeToken, "x")
	moved := idOf(app.connect(root, other, f, nil))
	execSQL(t, app.db, "UPDATE forge_connections SET credentials_enc = $1 WHERE id = $2", enc, moved)
	r = app.call("POST", connectionPath(other, moved)+"/check", root)
	eq(t, code(t, r), any("forge.credentials_unavailable"), r.text())

	ref := app.send("POST", connectionsPath(other), root, obj{"kind": "gitea", "api_url": "https://code.example", "owner_path": "acme",
		"credentials": obj{"token_ref": "env:FORGE_TOKEN_ACME"}})
	eq(t, ref.status, 201, ref.text())
	eq(t, at(ref.json(t), "credentials", "ref"), any("env:FORGE_TOKEN_ACME"))
	eq(t, at(ref.json(t), "credentials", "kind"), any("ref"))
}

func TestConnectionRules(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	project := app.nodeID(root, "project", acme, "p")
	ok := obj{"kind": "github", "owner_path": "acme-inc", "credentials": obj{"token": "t"}}
	with := func(k string, v any) obj {
		b := obj{}
		for kk, vv := range ok {
			b[kk] = vv
		}
		b[k] = v
		return b
	}
	tests := []struct {
		node string
		body obj
		code string
	}{
		{project, ok, "validation.invalid_forge_container"},
		{acme, with("kind", "svn"), "validation.invalid_forge"},
		{acme, with("api_url", "ftp://x"), "validation.invalid_api_url"},
		{acme, with("owner_path", "a/b"), "validation.invalid_owner_path"},
		{acme, with("name_include", []string{"[x"}), "validation.invalid_name_patterns"},
		{acme, with("interval_secs", 30), "validation.invalid_sync_interval"},
		{acme, with("credentials", obj{"token": "a", "token_ref": "env:X"}), "validation.invalid_credentials"},
		{acme, with("credentials", obj{"token_ref": "vault:x"}), "validation.invalid_credentials"},
		{acme, with("kind", "gitea"), "validation.invalid_api_url"},
	}
	for _, tt := range tests {
		r := app.send("POST", connectionsPath(tt.node), root, tt.body)
		eq(t, r.status, 400, tt.code, r.text())
		eq(t, code(t, r), any(tt.code))
	}
	eq(t, app.send("POST", connectionsPath(acme), root, ok).status, 201)
	globex := app.nodeID(root, "organization", "", "globex")
	r := app.send("POST", connectionsPath(globex), root, with("owner_path", "ACME-inc"))
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.forge_owner_taken"))

	noKey := startForgeApp(t, "SECRETS_KEYS", "")
	nk := noKey.admin()
	org := noKey.nodeID(nk, "organization", "", "o")
	r = noKey.send("POST", connectionsPath(org), nk, ok)
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.secrets_key_missing"))
	eq(t, noKey.send("POST", connectionsPath(org), nk, with("credentials", obj{"token_ref": "env:X"})).status, 201, "references need no key")
}

func TestConnectionRights(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api"})
	conn := idOf(app.connect(root, acme, f, nil))
	for email, role := range map[string]string{"viewer@example.com": "viewer", "editor@example.com": "editor", "admin@example.com": "admin"} {
		app.user(email, false)
		app.grant(root, acme, "user", email, role)
	}
	viewer := app.session("viewer@example.com", password)
	editor := app.session("editor@example.com", password)
	admin := app.session("admin@example.com", password)

	eq(t, app.get(connectionsPath(acme), viewer).status, 200)
	eq(t, app.get(connectionPath(acme, conn)+"/runs", viewer).status, 200)
	eq(t, code(t, app.call("POST", connectionPath(acme, conn)+"/sync", viewer)), any("auth.forbidden"))
	eq(t, app.call("POST", connectionPath(acme, conn)+"/sync", editor).status, 202)
	eq(t, app.call("POST", connectionPath(acme, conn)+"/check", editor).status, 200)
	eq(t, code(t, app.send("PATCH", connectionPath(acme, conn), editor, obj{"interval_secs": 120})), any("auth.forbidden"))
	eq(t, code(t, app.send("POST", connectionsPath(acme), editor, obj{"kind": "github", "owner_path": "x", "credentials": obj{"token": "t"}})), any("auth.forbidden"))
	r := app.send("PATCH", connectionPath(acme, conn), admin, obj{"interval_secs": 120})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["interval_secs"], any(float64(120)))
	eq(t, at(r.json(t), "credentials", "kind"), any("token"), "credentials kept")

	app.user("stranger@example.com", false)
	stranger := app.session("stranger@example.com", password)
	eq(t, app.get(connectionsPath(acme), stranger).status, 404)
	other := app.nodeID(root, "organization", "", "other")
	eq(t, app.get(connectionPath(other, conn), root).status, 404, "connection of another node")

	read := app.pat(admin, "read")
	eq(t, app.bearer("GET", connectionsPath(acme), read, nil).status, 200)
	eq(t, code(t, app.bearer("POST", connectionPath(acme, conn)+"/sync", read, nil)), any("auth.insufficient_scope"))
	write := app.pat(admin, "write")
	eq(t, app.bearer("POST", connectionPath(acme, conn)+"/sync", write, nil).status, 202)
}

func TestCheckAndPreview(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	app.node(root, "project", acme, "web")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api"})
	f.Put(forgefake.Repo{ID: 2, Path: "acme-inc/web"})
	conn := idOf(app.connect(root, acme, f, nil))
	r := app.call("POST", connectionPath(acme, conn)+"/check", root)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["ok"], any(true))

	nodes := app.count("nodes")
	p := app.call("POST", connectionPath(acme, conn)+"/preview", root)
	eq(t, p.status, 200, p.text())
	actions := map[string]string{}
	for _, it := range list(t, p.json(t), "items") {
		item := it.(obj)
		actions[item["full_path"].(string)] = item["action"].(string)
		if item["action"] == "skip" {
			eq(t, item["code"], any("conflict.slug_taken"))
		}
	}
	eq(t, actions["acme-inc/api"], "create")
	eq(t, actions["acme-inc/web"], "skip")
	eq(t, app.count("nodes"), nodes, "preview changes nothing")

	app.syncNow(root, acme, conn)
	f.Delete(1)
	p = app.call("POST", connectionPath(acme, conn)+"/preview", root)
	eq(t, at(p.json(t), "items", len(list(t, p.json(t), "items"))-1, "action"), any("orphan"))

	bad := idOf(app.connect(root, app.nodeID(root, "organization", "", "b"), forgefake.Start(t, "gitlab", forgeToken, "acme"),
		obj{"credentials": obj{"token": "nope"}}))
	bnode := scalar[string](t, app.db, "SELECT node_id::text FROM forge_connections WHERE id = $1", bad)
	r = app.call("POST", connectionPath(bnode, bad)+"/check", root)
	eq(t, r.status, 422)
	eq(t, code(t, r), any("forge.unauthorized"))
	missing := idOf(app.connect(root, app.nodeID(root, "organization", "", "m"), forgefake.Start(t, "github", forgeToken, "acme-inc"),
		obj{"owner_path": "nobody"}))
	mnode := scalar[string](t, app.db, "SELECT node_id::text FROM forge_connections WHERE id = $1", missing)
	eq(t, code(t, app.call("POST", connectionPath(mnode, missing)+"/check", root)), any("forge.owner_not_found"))
	down := idOf(app.connect(root, app.nodeID(root, "organization", "", "d"), forgefake.Start(t, "github", forgeToken, "down"),
		obj{"api_url": "http://127.0.0.1:1"}))
	dnode := scalar[string](t, app.db, "SELECT node_id::text FROM forge_connections WHERE id = $1", down)
	r = app.call("POST", connectionPath(dnode, down)+"/check", root)
	eq(t, r.status, 502)
	eq(t, code(t, r), any("forge.upstream_error"))
}

func TestSyncRunsAreListedAndPruned(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t, "FORGE_SYNC_RUNS_KEPT", "2")
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	conn := idOf(app.connect(root, acme, f, nil))
	r := app.call("POST", connectionPath(acme, conn)+"/sync", root)
	eq(t, r.status, 202)
	eq(t, app.call("POST", connectionPath(acme, conn)+"/sync", root).status, 202)
	eq(t, app.runDue(), 1, "a repeated request before the start runs once")
	app.syncNow(root, acme, conn)
	app.syncNow(root, acme, conn)
	runs := list(t, app.get(connectionPath(acme, conn)+"/runs", root).json(t), "items")
	eq(t, len(runs), 2)
	eq(t, at(app.get(connectionPath(acme, conn), root).json(t), "last_run", "status"), any("succeeded"))
}

func TestDeletingAConnectionUnlinksItsProjects(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api"})
	conn := idOf(app.connect(root, acme, f, nil))
	app.syncNow(root, acme, conn)
	api := idOf(app.childBySlug(root, acme, "api"))
	eq(t, app.call("DELETE", connectionPath(acme, conn), root).status, 204)
	eq(t, app.get(connectionPath(acme, conn), root).status, 404)
	got := app.get(nodePath(api), root).json(t)
	eq(t, got["managed"], any(false))
	eq(t, got["repository"], nil)
	eq(t, app.call("DELETE", nodePath(api), root).status, 204)
}
