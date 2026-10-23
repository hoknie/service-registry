package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
)

func TestSubscriptionsOnNodes(t *testing.T) {
	t.Parallel()
	app := startNotifyApp(t, nil)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	backend := app.nodeID(root, "folder", acme, "backend")
	web := app.nodeID(root, "folder", acme, "web")
	slack := app.secret(root, backend, obj{"value": "https://hooks.slack.com/services/T/B/x"})

	got := app.mustSubscribe(root, backend, "ops", obj{"channel": "slack", "secret_id": slack, "event_types": []string{"deployment.created"}})
	eq(t, got["key"], any("ops"))
	eq(t, got["signing"], nil)
	eq(t, at(got, "secret", "id"), any(slack))
	eq(t, got["locale"], any("en"))
	if strings.Contains(jsonText(got), "hooks.slack.com") {
		t.Fatalf("the address leaked: %v", got)
	}

	wrong := app.secret(root, backend, obj{"value": "https://chat.example/hook"})
	r := app.subscribe(root, backend, "chat", obj{"channel": "slack", "secret_id": wrong, "event_types": []string{"deployment.created"}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.invalid_notification_url"))

	neighbour := app.secret(root, web, obj{"value": "https://hooks.example/web"})
	r = app.subscribe(root, backend, "web", obj{"channel": "webhook", "secret_id": neighbour, "event_types": []string{"deployment.created"}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.secret_not_available"))

	r = app.subscribe(root, backend, "bad", obj{"channel": "email", "secret_id": slack, "event_types": []string{"deployment.created"}})
	eq(t, code(t, r), any("validation.invalid_subscription"))
	r = app.subscribe(root, backend, "bad", obj{"channel": "webhook", "secret_id": slack, "event_types": []string{"test"}})
	eq(t, code(t, r), any("validation.invalid_subscription"))
	r = app.subscribe(root, backend, "bad", obj{"channel": "webhook", "secret_id": slack, "event_types": []string{"link.check_recovered"}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.recovery_without_failure"))
	r = app.subscribe(root, backend, "paired", obj{"channel": "webhook", "secret_id": slack,
		"event_types": []string{"link.check_failed", "link.check_recovered"}})
	eq(t, r.status, 200)
	r = app.subscribe(root, backend, "paired", obj{"channel": "webhook", "secret_id": slack,
		"event_types": []string{"deployment.created", "link.check_recovered"}})
	eq(t, r.status, 400)
	eq(t, code(t, r), any("validation.recovery_without_failure"))
	eq(t, app.send("DELETE", subscriptionsPath(backend)+"/paired", root, nil).status, 204)

	app.mustSubscribe(root, acme, "audit", app.hook(root, acme, "https://hooks.example/audit"))
	items := list(t, app.get(subscriptionsPath(backend), root).json(t), "items")
	eq(t, len(items), 2)
	eq(t, items[0].(obj)["key"], any("audit"))
	eq(t, items[0].(obj)["inherited"], any(true))
	eq(t, items[0].(obj)["node_id"], any(acme))
	eq(t, items[1].(obj)["inherited"], any(false))

	app.mustSubscribe(root, backend, "audit", obj{"channel": "webhook", "event_types": []string{"deployment.created"}, "disabled": true})
	items = list(t, app.get(subscriptionsPath(backend), root).json(t), "items")
	eq(t, items[0].(obj)["inherited"], any(false))
	eq(t, items[0].(obj)["disabled"], any(true))
	eq(t, app.call("DELETE", subscriptionsPath(backend)+"/audit", root).status, 204)
	eq(t, app.call("DELETE", subscriptionsPath(backend)+"/audit", root).status, 404)

	editor := app.user("editor@example.com", false)
	app.grant(root, backend, "user", editor.Email, "editor")
	ed := app.session(editor.Email, password)
	r = app.get(subscriptionsPath(backend), ed)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.forbidden"))
	eq(t, app.get(deliveriesPath(backend), ed).status, 403)

	var usedBy any
	for _, it := range list(t, app.get(secretsPath(backend), root).json(t), "items") {
		if it.(obj)["id"] == slack {
			usedBy = it.(obj)["used_by"]
		}
	}
	eq(t, usedBy, any(float64(1)))
	del := app.call("DELETE", secretsPath(backend)+"/"+slack, root)
	eq(t, del.status, 409)
	eq(t, code(t, del), any("conflict.secret_in_use"))

	for i := 0; i < 19; i++ {
		app.mustSubscribe(root, web, "s"+strconv.Itoa(i), obj{"channel": "webhook", "event_types": []string{"deployment.created"}, "disabled": true})
	}
	app.mustSubscribe(root, web, "s19", obj{"channel": "webhook", "event_types": []string{"deployment.created"}, "disabled": true})
	r = app.subscribe(root, web, "s20", obj{"channel": "webhook", "event_types": []string{"deployment.created"}, "disabled": true})
	eq(t, code(t, r), any("validation.too_many_subscriptions"))
}

func TestSubscriptionTokenScopes(t *testing.T) {
	t.Parallel()
	app := startNotifyApp(t, nil)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	owner := app.user("owner@example.com", false)
	app.grant(root, acme, "user", owner.Email, "admin")
	cookie := app.session(owner.Email, password)
	read := app.pat(cookie, "read")
	write := app.pat(cookie, "write")
	body := app.hook(root, acme, "https://hooks.example/x")
	eq(t, app.bearer("GET", subscriptionsPath(acme), read, nil).status, 200)
	r := app.bearer("PUT", subscriptionsPath(acme)+"/hooks", read, body)
	eq(t, r.status, 403)
	eq(t, code(t, r), any("auth.insufficient_scope"))
	eq(t, app.bearer("PUT", subscriptionsPath(acme)+"/hooks", write, body).status, 200)
	eq(t, app.bearer("GET", deliveriesPath(acme), read, nil).status, 200)
}

func TestSigningSecretAndTestSend(t *testing.T) {
	t.Parallel()
	app := startNotifyApp(t, nil)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	rc := newReceiver(t, 204, 503)
	app.mustSubscribe(root, acme, "hooks", app.hook(root, acme, rc.URL+"/hook"))

	r := app.call("POST", subscriptionsPath(acme)+"/hooks/signing-secret", root)
	eq(t, r.status, 200, r.text())
	secret := r.json(t)["secret"].(string)
	if !strings.HasPrefix(secret, "whsec_") {
		t.Fatalf("secret %q", secret)
	}
	item := list(t, app.get(subscriptionsPath(acme), root).json(t), "items")[0].(obj)
	eq(t, at(item, "signing", "fingerprint"), r.json(t)["fingerprint"])
	if strings.Contains(jsonText(item), secret) {
		t.Fatal("the signing key leaked")
	}

	r = app.call("POST", subscriptionsPath(acme)+"/hooks/test", root)
	eq(t, r.status, 200, r.text())
	res := r.json(t)
	eq(t, res["status"], any("delivered"))
	eq(t, res["http_status"], any(float64(204)))
	eq(t, res["error_code"], nil)
	got := rc.requests()[0]
	key, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(got.header.Get("webhook-id") + "." + got.header.Get("webhook-timestamp") + "."))
	mac.Write(got.body)
	eq(t, got.header.Get("webhook-signature"), "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	eq(t, got.header.Get("Content-Type"), "application/json")
	if !strings.HasPrefix(got.header.Get("User-Agent"), "svc-registry/") {
		t.Fatalf("user agent %q", got.header.Get("User-Agent"))
	}
	p := rc.payload(t, 0)
	eq(t, p["type"], any("test"))
	eq(t, at(p, "data", "subscription"), any("hooks"))
	eq(t, at(p, "node", "path"), any("acme"))
	eq(t, at(p, "node", "url"), any("https://registry.example/en/catalog?node="+acme))

	r = app.call("POST", subscriptionsPath(acme)+"/hooks/test", root)
	eq(t, r.status, 429)
	eq(t, code(t, r), any("notify.rate_limited"))
	if n, err := strconv.Atoi(r.header("Retry-After")); err != nil || n < 1 || n > 10 {
		t.Fatalf("Retry-After %q", r.header("Retry-After"))
	}

	journal := app.deliveriesOf(root, acme, "")
	eq(t, len(journal), 1)
	d := journal[0].(obj)
	eq(t, d["event_type"], any("test"))
	eq(t, d["status"], any("delivered"))
	eq(t, d["subscription"], any("hooks"))
	eq(t, d["attempts"], any(float64(1)))
	eq(t, at(d, "event_node", "path"), any("acme"))
	eq(t, len(app.deliveriesOf(root, acme, "?status=failed")), 0)
	bad := app.get(deliveriesPath(acme)+"?status=lost", root)
	eq(t, bad.status, 400)
	eq(t, code(t, bad), any("validation.invalid_filter"))

	app.mustSubscribe(root, acme, "chat", obj{"channel": "slack", "secret_id": app.secret(root, acme,
		obj{"value": "https://hooks.slack.com/services/T/B/x"}), "event_types": []string{"deployment.created"}})
	eq(t, app.call("POST", subscriptionsPath(acme)+"/chat/signing-secret", root).status, 404)
	eq(t, app.call("POST", subscriptionsPath(acme)+"/none/signing-secret", root).status, 404)

	noKeys := startNotifyApp(t, nil, "SECRETS_KEYS", "")
	nroot := noKeys.admin()
	org := noKeys.nodeID(nroot, "organization", "", "acme")
	noKeys.mustSubscribe(nroot, org, "hooks", obj{"channel": "webhook", "event_types": []string{"deployment.created"}, "disabled": true})
	r = noKeys.call("POST", subscriptionsPath(org)+"/hooks/signing-secret", nroot)
	eq(t, r.status, 409)
	eq(t, code(t, r), any("conflict.secrets_key_missing"))
}

func TestSigningKeysRotateWithSecrets(t *testing.T) {
	t.Parallel()
	app := startNotifyApp(t, nil)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	rc := newReceiver(t)
	app.mustSubscribe(root, acme, "hooks", app.hook(root, acme, rc.URL))
	secret := app.call("POST", subscriptionsPath(acme)+"/hooks/signing-secret", root).json(t)["secret"].(string)
	code, out, errOut := run(t, []string{"secrets:rotate"}, "", "DATABASE_URL", app.db.URL, "SECRETS_KEYS", secretsKey2+","+secretsKey)
	eq(t, code, 0, errOut)
	contains(t, out, "rotated")
	app.restart("SECRETS_KEYS", secretsKey2)
	eq(t, app.call("POST", subscriptionsPath(acme)+"/hooks/test", root).json(t)["status"], any("delivered"))
	got := rc.requests()[0]
	key, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(got.header.Get("webhook-id") + "." + got.header.Get("webhook-timestamp") + "."))
	mac.Write(got.body)
	eq(t, got.header.Get("webhook-signature"), "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}
