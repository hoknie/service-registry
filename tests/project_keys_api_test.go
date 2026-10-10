package tests

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/google/uuid"

	"svc-registry/internal/platform/auth"
)

func keyProject(app *testApp, admin string) (string, obj) {
	acme := app.nodeID(admin, "organization", "", "acme")
	p := app.node(admin, "project", acme, "api")
	return idOf(p), p
}

func (a *testApp) verifies(project, key string) bool {
	a.t.Helper()
	id, err := a.services.Catalog.VerifyProjectKey(context.Background(), uuid.MustParse(project), key)
	if err != nil {
		a.t.Fatal(err)
	}
	return id != nil
}

func keysPath(project string) string { return nodePath(project) + "/keys" }

func (a *testApp) keys(cookie, project string) []any {
	a.t.Helper()
	r := a.get(keysPath(project), cookie)
	if r.status != 200 {
		a.t.Fatalf("keys: %d %s", r.status, r.body)
	}
	return list(a.t, r.json(a.t), "items")
}

func TestAProjectIsCreatedWithAKeyShownOnce(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, p := keyProject(app, admin)
	secret := at(p, "key", "secret").(string)
	eq(t, len(secret), 43)
	eq(t, strings.HasPrefix(secret, "svcr_"), true)
	for _, c := range secret[5:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			t.Fatalf("not alphanumeric: %s", secret)
		}
	}
	eq(t, at(p, "key", "prefix"), any(secret[:12]))
	eq(t, at(p, "key", "status"), any("active"))

	var prefix string
	var hash []byte
	if err := app.db.Pool.QueryRow(context.Background(), "SELECT prefix, key_hash FROM project_keys").Scan(&prefix, &hash); err != nil {
		t.Fatal(err)
	}
	eq(t, prefix, secret[:12])
	sum := sha256.Sum256([]byte(secret))
	eq(t, string(hash), string(sum[:]))
	eq(t, scalar[int64](t, app.db, "SELECT count(*) FROM project_keys k WHERE k::text LIKE '%' || $1 || '%'", secret), int64(0))

	keys := app.keys(admin, pid)
	eq(t, len(keys), 1)
	k := keys[0].(obj)
	eq(t, has(k, "secret"), false, "never shown again")
	eq(t, k["status"], any("active"))
	eq(t, has(k, "expires_at") && k["expires_at"] == nil, true)
	eq(t, has(k, "last_used_at") && k["last_used_at"] == nil, true)

	eq(t, app.verifies(pid, secret), true)
	eq(t, at(app.keys(admin, pid)[0], "last_used_at") != nil, true, "use is recorded")
}

func TestKeysNeedCatalogKeysOnAProject(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, _ := keyProject(app, admin)
	bob := bobSession(app)
	app.grant(admin, pid, "user", "bob@example.com", "viewer")
	r := app.get(keysPath(pid), bob)
	eq(t, r.status, 403)
	eq(t, r.json(t)["code"], any("auth.forbidden"))
	eq(t, app.send("POST", keysPath(pid), bob, obj{}).status, 403)

	app.grant(admin, pid, "user", "bob@example.com", "editor")
	eq(t, len(app.keys(bob, pid)), 1)
	org := at(app.get("/api/v1/catalog/nodes", admin).json(t), "items", 0, "id").(string)
	eq(t, app.get(keysPath(org), admin).status, 404)
}

func TestRotationKeepsOlderKeysForTheGracePeriod(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, p := keyProject(app, admin)
	a := at(p, "key", "secret").(string)

	r := app.send("POST", keysPath(pid), admin, obj{"grace_secs": 3600})
	eq(t, r.status, 201, r.text())
	b := r.json(t)["secret"].(string)
	eq(t, a != b, true)
	keys := app.keys(admin, pid)
	eq(t, len(keys), 2)
	eq(t, at(keys[0], "prefix"), any(b[:12]), "newest first")
	eq(t, at(keys[0], "expires_at"), any(nil))
	eq(t, at(keys[1], "status"), any("active"))
	eq(t, scalar[bool](t, app.db, "SELECT bool_and(expires_at BETWEEN now() + interval '59 minutes' AND now() + interval '61 minutes') "+
		"FROM project_keys WHERE expires_at IS NOT NULL"), true)
	eq(t, app.verifies(pid, a), true)
	eq(t, app.verifies(pid, b), true)

	r = app.call("POST", keysPath(pid), admin)
	eq(t, r.status, 201, r.text())
	eq(t, app.verifies(pid, a), true)

	d := app.send("POST", keysPath(pid), admin, obj{"grace_secs": 0}).json(t)["secret"].(string)
	eq(t, app.verifies(pid, a), false)
	eq(t, app.verifies(pid, b), false)
	eq(t, app.verifies(pid, d), true)
	expired := 0
	for _, k := range app.keys(admin, pid) {
		if at(k, "status") == "expired" {
			expired++
		}
	}
	eq(t, expired, 3)

	for _, bad := range []obj{{"grace_secs": -1}, {"grace_secs": 604_801}} {
		r := app.send("POST", keysPath(pid), admin, bad)
		eq(t, r.status, 400)
		eq(t, r.json(t)["code"], any("validation.invalid_grace_period"))
	}
	for _, bad := range []string{`{"grace_secs": 1.5}`, `{"grace_secs": "1"}`, `nope`} {
		r := request(t, app.addr, "POST", keysPath(pid), bad, "cookie", admin)
		eq(t, r.status, 400, bad)
		eq(t, r.json(t)["code"], any("validation.invalid_body"), bad)
	}
}

func TestRevokedKeysStopVerifyingAtOnce(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, p := keyProject(app, admin)
	secret := at(p, "key", "secret").(string)
	kid := at(p, "key", "id").(string)
	for range 2 {
		eq(t, app.call("DELETE", keysPath(pid)+"/"+kid, admin).status, 204, "idempotent")
	}
	k := app.keys(admin, pid)[0].(obj)
	eq(t, k["status"], any("revoked"))
	eq(t, k["revoked_at"] != nil, true)
	eq(t, app.verifies(pid, secret), false)

	acme := at(app.get("/api/v1/catalog/nodes", admin).json(t), "items", 0, "id").(string)
	other := app.nodeID(admin, "project", acme, "other")
	eq(t, app.call("DELETE", keysPath(other)+"/"+kid, admin).status, 404)
	eq(t, app.call("DELETE", keysPath(pid)+"/"+newID(), admin).status, 404)
}

func TestVerificationRejectsForeignAndTamperedKeys(t *testing.T) {
	t.Parallel()
	app := startApp(t)
	admin := app.admin()
	pid, p := keyProject(app, admin)
	secret := at(p, "key", "secret").(string)
	other := app.nodeID(admin, "project", p["parent_id"].(string), "other")

	eq(t, app.verifies(pid, secret), true)
	eq(t, app.verifies(other, secret), false, "another project's key")
	tampered := []byte(secret)
	if tampered[10] == 'A' {
		tampered[10] = 'B'
	} else {
		tampered[10] = 'A'
	}
	_, ok := auth.ParseProjectKey(string(tampered))
	eq(t, ok, false, "rejected before the database")
	eq(t, app.verifies(pid, string(tampered)), false)
	stranger, err := auth.GenerateProjectKey()
	if err != nil {
		t.Fatal(err)
	}
	eq(t, app.verifies(pid, stranger.Secret), false)
}
