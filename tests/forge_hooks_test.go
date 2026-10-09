package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"svc-registry/internal/testsupport/forgefake"
)

func hmacHex(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *testApp) manualHook(cookie, node, conn string) (string, string) {
	a.t.Helper()
	r := a.send("POST", connectionPath(node, conn)+"/webhook", cookie, obj{"mode": "manual"})
	if r.status != 200 {
		a.t.Fatalf("webhook: %d %s", r.status, r.body)
	}
	v := r.json(a.t)
	return v["secret"].(string), v["url"].(string)
}

func TestSignedDeliveriesQueueARun(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	body := `{"ref":"refs/heads/main"}`
	for _, kind := range []string{"github", "gitea", "forgejo", "gitlab"} {
		t.Run(kind, func(t *testing.T) {
			org := app.nodeID(root, "organization", "", "org-"+kind)
			f := forgefake.Start(t, kind, forgeToken, "acme-"+kind)
			conn := idOf(app.connect(root, org, f, nil))
			secret, url := app.manualHook(root, org, conn)
			eq(t, url, "https://registry.example/api/v1/forge/hooks/"+conn)
			eq(t, len(secret), 64)
			got := app.get(connectionPath(org, conn), root)
			eq(t, at(got.json(t), "webhook", "mode"), any("manual"))
			eq(t, at(got.json(t), "webhook", "url"), any(url))
			lacks(t, got.text(), secret)

			var headers []string
			switch kind {
			case "github":
				headers = []string{"X-Hub-Signature-256", "sha256=" + hmacHex(secret, body)}
			case "gitea":
				headers = []string{"X-Gitea-Signature", hmacHex(secret, body)}
			case "forgejo":
				headers = []string{"X-Forgejo-Signature", hmacHex(secret, body)}
			case "gitlab":
				headers = []string{"X-Gitlab-Token", secret}
			}
			path := "/api/v1/forge/hooks/" + conn
			r := request(t, app.addr, "POST", path, body, append([]string{"content-type", "application/json", "origin", "https://github.com"}, headers...)...)
			eq(t, r.status, 202, r.text())
			eq(t, scalar[string](t, app.db, "SELECT next_trigger FROM forge_connections WHERE id = $1", conn), "webhook")
			app.runDue()
			eq(t, app.lastRun(root, org, conn)["trigger"], any("webhook"))

			headers[1] = strings.Repeat("0", len(headers[1]))
			r = request(t, app.addr, "POST", path, body, append([]string{"sec-fetch-site", "cross-site"}, headers...)...)
			eq(t, r.status, 401)
			eq(t, code(t, r), any("auth.invalid_webhook_signature"))
		})
	}
}

func TestDeliveriesToUnknownOrUnsetConnectionsAreRefusedAlike(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	conn := idOf(app.connect(root, acme, forgefake.Start(t, "github", forgeToken, "acme-inc"), nil))
	for _, path := range []string{"/api/v1/forge/hooks/" + conn, "/api/v1/forge/hooks/" + newID(), "/api/v1/forge/hooks/not-a-uuid"} {
		r := request(t, app.addr, "POST", path, "{}", "X-Hub-Signature-256", "sha256="+hmacHex("x", "{}"))
		eq(t, r.status, 401, path)
		eq(t, code(t, r), any("auth.invalid_webhook_signature"))
	}
	r := request(t, app.addr, "POST", "/api/v1/forge/hooks/"+conn, strings.Repeat("a", 2<<20))
	eq(t, r.status, 413)
	eq(t, code(t, r), any("validation.invalid_body"))
	r = app.call("GET", "/api/v1/forge/hooks/"+conn, "")
	eq(t, r.status, 405)
	eq(t, r.header("Allow"), "POST")
}

func TestWebhookRegistrationOnTheForge(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	f := forgefake.Start(t, "github", forgeToken, "acme-inc")
	conn := idOf(app.connect(root, acme, f, nil))
	r := app.send("POST", connectionPath(acme, conn)+"/webhook", root, obj{"mode": "register"})
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["mode"], any("register"))
	eq(t, has(r.json(t), "secret"), false)
	hooks := f.Hooks()
	eq(t, len(hooks), 1)
	eq(t, hooks[0].URL, "https://registry.example/api/v1/forge/hooks/"+conn)
	body := `{"zen":"x"}`
	d := request(t, app.addr, "POST", "/api/v1/forge/hooks/"+conn, body, "X-Hub-Signature-256", "sha256="+hmacHex(hooks[0].Secret, body))
	eq(t, d.status, 202, d.text())

	eq(t, app.send("POST", connectionPath(acme, conn)+"/webhook", root, obj{"mode": "register"}).status, 200)
	eq(t, len(f.Hooks()), 1)
	eq(t, f.Hooks()[0].Secret != hooks[0].Secret, true)

	eq(t, app.call("DELETE", connectionPath(acme, conn)+"/webhook", root).status, 204)
	eq(t, len(f.Hooks()), 0)
	eq(t, app.get(connectionPath(acme, conn), root).json(t)["webhook"], nil)

	f.FailHooks(http.StatusForbidden)
	r = app.send("POST", connectionPath(acme, conn)+"/webhook", root, obj{"mode": "register"})
	eq(t, r.status, 502)
	eq(t, code(t, r), any("forge.upstream_error"))
	eq(t, app.get(connectionPath(acme, conn), root).json(t)["webhook"], nil, "unchanged on failure")
	eq(t, code(t, app.send("POST", connectionPath(acme, conn)+"/webhook", root, obj{"mode": "push"})), any("validation.invalid_webhook_mode"))

	f.FailHooks(0)
	eq(t, app.send("POST", connectionPath(acme, conn)+"/webhook", root, obj{"mode": "register"}).status, 200)
	eq(t, app.call("DELETE", connectionPath(acme, conn), root).status, 204)
	eq(t, len(f.Hooks()), 0)
}

func TestWebhookNeedsAPublicURL(t *testing.T) {
	t.Parallel()
	app := startForgeApp(t, "PUBLIC_URL", "")
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	conn := idOf(app.connect(root, acme, forgefake.Start(t, "github", forgeToken, "acme-inc"), nil))
	r := app.send("POST", connectionPath(acme, conn)+"/webhook", root, obj{"mode": "manual"})
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.public_url_missing"))
}
