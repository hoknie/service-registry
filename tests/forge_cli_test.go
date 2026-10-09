package tests

import (
	"strings"
	"testing"

	"svc-registry/internal/testsupport/forgefake"
)

const secretsKey2 = "k2:YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU="

func TestHelpListsTheForgeCommands(t *testing.T) {
	code, out, _ := run(t, nil, "")
	eq(t, code, 0)
	contains(t, out, "forge:sync")
	contains(t, out, "secrets:rotate")
}

func TestForgeSyncRunsOnceFromTheCommandLine(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	f.Put(forgefake.Repo{ID: 1, Path: "acme-inc/api"})
	conn := idOf(app.connect(root, acme, f, nil))

	code, out, errOut := run(t, []string{"forge:sync", conn}, "", "DATABASE_URL", app.db.URL, "SECRETS_KEYS", secretsKey)
	eq(t, code, 0, errOut)
	eq(t, strings.TrimSpace(out), "succeeded created=1 updated=0 orphaned=0 skipped=0")
	eq(t, app.lastRun(root, acme, conn)["trigger"], any("cli"))

	code, _, errOut = run(t, []string{"forge:sync", newID()}, "", "DATABASE_URL", app.db.URL)
	eq(t, code, 1)
	contains(t, errOut, "no forge connection")
	code, _, _ = run(t, []string{"forge:sync", "nope"}, "", "DATABASE_URL", app.db.URL)
	eq(t, code, 2)

	execSQL(t, app.db, "UPDATE forge_connections SET lease_until = now() + interval '1 hour'")
	code, _, errOut = run(t, []string{"forge:sync", conn}, "", "DATABASE_URL", app.db.URL, "SECRETS_KEYS", secretsKey)
	eq(t, code, 1)
	contains(t, errOut, "sync already running")

	execSQL(t, app.db, "UPDATE forge_connections SET lease_until = NULL")
	code, out, _ = run(t, []string{"forge:sync", conn}, "", "DATABASE_URL", app.db.URL)
	eq(t, code, 1, "no key to decrypt the token")
	contains(t, out, "failed")
}

func TestSecretsRotate(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	conn := idOf(app.connect(root, acme, f, nil))
	app.manualHook(root, acme, conn)
	app.cluster(root, "prod", nil)
	manual := app.nodeID(root, "project", acme, "docs")
	r := app.send("PUT", nodePath(manual)+"/knowledge/source", root, obj{"kind": "remote", "forge": "github",
		"url": "https://github.example/acme-inc/docs", "api_url": f.APIURL(), "credentials": obj{"token": forgeToken}})
	eq(t, r.status, 200, r.text())

	code, out, errOut := run(t, []string{"secrets:rotate"}, "", "DATABASE_URL", app.db.URL, "SECRETS_KEYS", secretsKey2+","+secretsKey)
	eq(t, code, 0, errOut)
	eq(t, strings.TrimSpace(out), "rotated 4 secret(s)")
	eq(t, strings.HasPrefix(scalar[string](t, app.db, "SELECT credentials_enc FROM knowledge_sources"), "v1.k2."), true)
	eq(t, strings.HasPrefix(scalar[string](t, app.db, "SELECT credentials_enc FROM clusters"), "v1.k2."), true)
	eq(t, strings.HasPrefix(scalar[string](t, app.db, "SELECT credentials_enc FROM forge_connections"), "v1.k2."), true)
	eq(t, strings.HasPrefix(scalar[string](t, app.db, "SELECT webhook_secret_enc FROM forge_connections"), "v1.k2."), true)

	code, out, errOut = run(t, []string{"forge:sync", conn}, "", "DATABASE_URL", app.db.URL, "SECRETS_KEYS", secretsKey2)
	eq(t, code, 0, errOut+out)

	code, _, _ = run(t, []string{"secrets:rotate"}, "", "DATABASE_URL", app.db.URL, "SECRETS_KEYS", secretsKey)
	eq(t, code, 1)
	eq(t, strings.HasPrefix(scalar[string](t, app.db, "SELECT credentials_enc FROM forge_connections"), "v1.k2."), true)
}
