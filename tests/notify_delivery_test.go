package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"svc-registry/internal/presentation/jobs"
)

func init() { jobs.NotifyDeliveryTick = 50 * time.Millisecond }

func (n *notifyTree) deployOnce(key string) {
	n.t.Helper()
	if r := n.ingest(n.api, n.apiKey, deployed(key, "api", "production", "1.0."+key, time.Now().UTC().Format(time.RFC3339))); r.status != 201 {
		n.t.Fatalf("ingest: %d %s", r.status, r.body)
	}
}

func (n *notifyTree) delivery() obj {
	n.t.Helper()
	items := n.deliveriesOf(n.root, n.acme, "")
	if len(items) == 0 {
		n.t.Fatal("no deliveries")
	}
	return items[0].(obj)
}

func TestDeliveryRetriesUntilDelivered(t *testing.T) {
	t.Parallel()
	n := startNotifyTree(t, nil)
	rc := newReceiver(t, 503, 200)
	n.mustSubscribe(n.root, n.acme, "ops", n.hook(n.root, n.acme, rc.URL))
	n.deployOnce("1")
	eq(t, n.deliverNow(), 1)
	d := n.delivery()
	eq(t, d["status"], any("pending"))
	eq(t, d["attempts"], any(float64(1)))
	eq(t, d["last_http_status"], any(float64(503)))
	eq(t, d["last_error_code"], any("notify.http_status"))
	eq(t, scalar[bool](t, n.db, "SELECT next_run_at >= now() + interval '29 seconds' FROM notification_deliveries"), true)
	ctx := context.Background()
	ids, err := n.services.Notify.Deliveries.ClaimDeliveries(ctx, 10, 60)
	must(t, err)
	eq(t, len(ids), 0, "not due before the pause")
	eq(t, n.deliverNow(), 1)
	d = n.delivery()
	eq(t, d["status"], any("delivered"))
	eq(t, d["attempts"], any(float64(2)))
	eq(t, d["next_attempt_at"], nil)
	if d["delivered_at"] == nil {
		t.Fatal("no delivered_at")
	}
	got := rc.requests()
	eq(t, len(got), 2)
	eq(t, got[0].header.Get("webhook-id"), got[1].header.Get("webhook-id"))
	eq(t, got[0].header.Get("webhook-id"), d["id"].(string))
	if got[1].header.Get("webhook-timestamp") < got[0].header.Get("webhook-timestamp") {
		t.Fatal("the timestamp went back")
	}
	eq(t, got[0].header.Get("webhook-signature"), "", "no signing key, no signature")
	eq(t, n.deliverNow(), 0)
}

func TestDeliveryOutcomes(t *testing.T) {
	t.Parallel()
	n := startNotifyTree(t, nil, "NOTIFY_MAX_ATTEMPTS", "2")
	gone := newReceiver(t, 404)
	n.mustSubscribe(n.root, n.acme, "gone", n.hook(n.root, n.acme, gone.URL))
	moved := newReceiver(t, 302)
	n.mustSubscribe(n.root, n.acme, "moved", n.hook(n.root, n.acme, moved.URL))
	busy := newReceiver(t, 429, 500)
	busy.headers["Retry-After"] = "120"
	n.mustSubscribe(n.root, n.acme, "busy", n.hook(n.root, n.acme, busy.URL))
	n.deployOnce("1")
	eq(t, n.deliverNow(), 3)
	status := func(key string) string {
		return scalar[string](t, n.db, `SELECT d.status || ' ' || d.attempts || ' ' || coalesce(d.last_http_status::text, '-') || ' ' ||
			coalesce(d.last_error_code, '-') FROM notification_deliveries d JOIN notification_subscriptions s ON s.id = d.subscription_id
			WHERE s.key = $1`, key)
	}
	eq(t, status("gone"), "failed 1 404 notify.http_status")
	eq(t, status("moved"), "failed 1 302 notify.redirect")
	eq(t, status("busy"), "pending 1 429 notify.http_status")
	eq(t, scalar[bool](t, n.db, `SELECT d.next_run_at >= now() + interval '119 seconds' FROM notification_deliveries d
		JOIN notification_subscriptions s ON s.id = d.subscription_id WHERE s.key = 'busy'`), true, "Retry-After")
	eq(t, n.deliverNow(), 1)
	eq(t, status("busy"), "failed 2 500 notify.http_status")

	n.mustSubscribe(n.root, n.acme, "later", n.hook(n.root, n.acme, gone.URL))
	n.deployOnce("2")
	n.mustSubscribe(n.root, n.acme, "later", obj{"channel": "webhook", "event_types": []string{"deployment.created"}, "disabled": true})
	n.deliverNow()
	eq(t, scalar[string](t, n.db, `SELECT d.status || ' ' || d.last_error_code FROM notification_deliveries d
		JOIN notification_subscriptions s ON s.id = d.subscription_id WHERE s.key = 'later'`), "failed notify.subscription_disabled")
	eq(t, n.call("DELETE", subscriptionsPath(n.acme)+"/gone", n.root).status, 204)
	eq(t, scalar[int64](t, n.db, "SELECT count(*) FROM notification_deliveries WHERE last_http_status = 404"), int64(0), "deleted with the subscription")
}

func TestDeliveryToCloudMetadataIsBlocked(t *testing.T) {
	t.Parallel()
	app := startRepoSyncApp(t)
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	p := app.node(root, "project", acme, "api")
	app.mustSubscribe(root, acme, "meta", app.hook(root, acme, "http://169.254.169.254/latest/meta-data/"))
	eq(t, app.ingest(idOf(p), at(p, "key", "secret").(string), deployed("1", "api", "production", "1.0.0", time.Now().UTC().Format(time.RFC3339))).status, 201)
	eq(t, app.deliverNow(), 1)
	d := app.deliveriesOf(root, acme, "")[0].(obj)
	eq(t, d["status"], any("failed"))
	eq(t, d["last_error_code"], any("notify.blocked"))
	eq(t, d["last_http_status"], nil)
	r := app.call("POST", subscriptionsPath(acme)+"/meta/test", root)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["error_code"], any("notify.blocked"))
}

func TestDeliveryJobRespectsBackgroundJobs(t *testing.T) {
	t.Parallel()
	off := startNotifyTree(t, nil, "BACKGROUND_JOBS_ENABLED", "false")
	rc := newReceiver(t)
	off.mustSubscribe(off.root, off.acme, "ops", off.hook(off.root, off.acme, rc.URL))
	off.deployOnce("1")
	ctx, cancel := context.WithCancel(context.Background())
	done := jobs.SpawnNotifyDeliveries(ctx, off.services.Jobs())
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	eq(t, off.delivery()["status"], any("pending"))
	eq(t, off.call("POST", subscriptionsPath(off.acme)+"/ops/test", off.root).json(t)["status"], any("delivered"))

	on := startNotifyTree(t, nil, "BACKGROUND_JOBS_ENABLED", "true")
	rc2 := newReceiver(t)
	on.mustSubscribe(on.root, on.acme, "ops", on.hook(on.root, on.acme, rc2.URL))
	on.deployOnce("1")
	ctx, cancel = context.WithCancel(context.Background())
	done = jobs.SpawnNotifyDeliveries(ctx, on.services.Jobs())
	deadline := time.Now().Add(5 * time.Second)
	for len(rc2.requests()) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	eq(t, len(rc2.requests()), 1)
}

func TestTwoReplicasDeliverOnce(t *testing.T) {
	t.Parallel()
	n := startNotifyTree(t, nil)
	rc := newReceiver(t)
	n.mustSubscribe(n.root, n.acme, "ops", n.hook(n.root, n.acme, rc.URL))
	for _, k := range []string{"1", "2", "3", "4", "5", "6"} {
		n.deployOnce(k)
	}
	second := appOverWith(t, n.db, n.opts, n.env...)
	var wg sync.WaitGroup
	for _, a := range []*testApp{n.testApp, second} {
		wg.Go(func() {
			ctx := context.Background()
			ids, err := a.services.Notify.Deliveries.ClaimDeliveries(ctx, 100, 60)
			if err != nil {
				t.Error(err)
				return
			}
			for _, id := range ids {
				if err := a.services.Notify.Deliveries.RunDelivery(ctx, id); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Wait()
	eq(t, len(rc.requests()), 6)
	eq(t, scalar[int64](t, n.db, "SELECT count(*) FROM notification_deliveries WHERE status = 'delivered' AND attempts = 1"), int64(6))
}

func TestDeliveryThroughTheProxy(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var seen []string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.String())
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(proxy.Close)
	app := startRepoSyncApp(t, "NOTIFY_USE_PROXY", "true", "HTTP_PROXY", proxy.URL, "NOTIFY_ALLOW_PRIVATE", "false")
	root := app.admin()
	acme := app.nodeID(root, "organization", "", "acme")
	app.mustSubscribe(root, acme, "named", app.hook(root, acme, "http://hooks.example/x"))
	app.mustSubscribe(root, acme, "literal", app.hook(root, acme, "http://127.0.0.1:8080/hook"))
	r := app.call("POST", subscriptionsPath(acme)+"/named/test", root)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["status"], any("delivered"))
	r = app.call("POST", subscriptionsPath(acme)+"/literal/test", root)
	eq(t, r.status, 200, r.text())
	eq(t, r.json(t)["error_code"], any("notify.blocked"))
	mu.Lock()
	defer mu.Unlock()
	eq(t, len(seen), 1)
	eq(t, seen[0], "http://hooks.example/x")
}
